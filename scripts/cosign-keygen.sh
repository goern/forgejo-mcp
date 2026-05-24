#!/usr/bin/env bash
#
# cosign-keygen.sh — generate a cosign keypair and emit a SOPS-encrypted
# Kubernetes Secret manifest. The private key and password never touch
# unencrypted disk outside a tmpfs working directory and are never echoed
# to stdout/stderr.
#
# Usage:
#   scripts/cosign-keygen.sh [namespace] [secret-name] [output-file]
#
# Defaults:
#   namespace   = forgejo-mcp
#   secret-name = cosign-signing-key
#   output-file = secrets/cosign-signing-key.enc.yaml
#
# Prerequisites:
#   - cosign  (https://github.com/sigstore/cosign)
#   - sops    (https://github.com/getsops/sops) with a working key in .sops.yaml
#   - A POSIX-ish shell with /dev/shm OR /tmp + shred(1)
#
# Output:
#   - $output-file          → SOPS-encrypted k8s Secret (cosign.key + cosign.password fields encrypted)
#   - dirname($out)/cosign.pub → plaintext public key (safe + intended to commit)
#
# Security guarantees:
#   - Private key + password live only in /dev/shm (RAM) when available;
#     fallback to /tmp with shred-on-exit (best effort).
#   - Password is read with `read -rs` (no terminal echo) and unset
#     immediately after the Secret YAML is built.
#   - Neither sops nor cosign output prints key material on success paths.
#   - `set -x` is never enabled. Do not add it.

set -euo pipefail

NAMESPACE="${1:-forgejo-mcp}"
SECRET_NAME="${2:-cosign-signing-key}"
OUT_FILE="${3:-secrets/cosign-signing-key.enc.yaml}"

# --- preflight ----------------------------------------------------------------

need() { command -v "$1" >/dev/null 2>&1 || { echo "missing dependency: $1" >&2; exit 2; }; }
need cosign
need sops
need base64
need sed
need mktemp

if [ -f "$OUT_FILE" ]; then
  printf 'Refuse to overwrite existing file: %s\n' "$OUT_FILE" >&2
  printf 'Move/delete it first if you really mean to regenerate.\n' >&2
  exit 3
fi

# --- tmpfs work dir -----------------------------------------------------------

if [ -d /dev/shm ] && [ -w /dev/shm ]; then
  WORKDIR=$(mktemp -d /dev/shm/cosign-keygen.XXXXXXXX)
  RAM_BACKED=1
else
  WORKDIR=$(mktemp -d)
  RAM_BACKED=0
  printf 'warning: /dev/shm unavailable; using %s (disk-backed). Will shred on exit.\n' "$WORKDIR" >&2
fi
chmod 700 "$WORKDIR"

cleanup() {
  if [ -d "$WORKDIR" ]; then
    if [ "$RAM_BACKED" = "0" ] && command -v shred >/dev/null 2>&1; then
      find "$WORKDIR" -type f -exec shred -uz {} + 2>/dev/null || true
    fi
    rm -rf "$WORKDIR"
  fi
  unset COSIGN_PASSWORD COSIGN_PASSWORD_CONFIRM 2>/dev/null || true
}
trap cleanup EXIT INT TERM HUP

# --- password (no echo) -------------------------------------------------------

printf 'Enter cosign key password (will not echo, min 12 chars): ' >&2
IFS= read -rs COSIGN_PASSWORD
printf '\n' >&2
if [ "${#COSIGN_PASSWORD}" -lt 12 ]; then
  printf 'password too short (need >=12 chars)\n' >&2
  exit 4
fi
printf 'Confirm: ' >&2
IFS= read -rs COSIGN_PASSWORD_CONFIRM
printf '\n' >&2
if [ "$COSIGN_PASSWORD" != "$COSIGN_PASSWORD_CONFIRM" ]; then
  printf 'passwords do not match\n' >&2
  exit 5
fi
unset COSIGN_PASSWORD_CONFIRM
export COSIGN_PASSWORD

# --- generate keypair into tmpfs (cosign writes cosign.key + cosign.pub) ------

# Redirect cosign's stdout: only the "Private key written to ..." path string
# would be shown, but suppress it entirely as defense-in-depth.
(cd "$WORKDIR" && cosign generate-key-pair) >/dev/null

if [ ! -s "$WORKDIR/cosign.key" ] || [ ! -s "$WORKDIR/cosign.pub" ]; then
  printf 'cosign did not produce expected files\n' >&2
  exit 6
fi
chmod 600 "$WORKDIR/cosign.key"

# --- build k8s Secret YAML in tmpfs ------------------------------------------

# Use stringData so kubectl handles the values verbatim. SOPS encrypts the
# sensitive fields (cosign.key + cosign.password) in place via --encrypted-regex.
{
  printf 'apiVersion: v1\n'
  printf 'kind: Secret\n'
  printf 'metadata:\n'
  printf '  name: %s\n' "$SECRET_NAME"
  printf '  namespace: %s\n' "$NAMESPACE"
  printf 'type: Opaque\n'
  printf 'stringData:\n'
  printf '  cosign.key: |\n'
  sed 's/^/    /' "$WORKDIR/cosign.key"
  printf '  cosign.password: %s\n' "$(printf '%s' "$COSIGN_PASSWORD" | sed 's/"/\\"/g; s/^/"/; s/$/"/')"
  printf '  cosign.pub: |\n'
  sed 's/^/    /' "$WORKDIR/cosign.pub"
} > "$WORKDIR/secret.yaml"

unset COSIGN_PASSWORD

# --- SOPS encrypt directly to destination -------------------------------------

OUT_DIR=$(dirname "$OUT_FILE")
mkdir -p "$OUT_DIR"

sops --encrypt \
  --input-type yaml \
  --output-type yaml \
  --encrypted-regex '^(cosign\.key|cosign\.password)$' \
  "$WORKDIR/secret.yaml" > "$OUT_FILE"

# Public key is intended to be committed and verified against. Emit alongside.
cp "$WORKDIR/cosign.pub" "$OUT_DIR/cosign.pub"
chmod 644 "$OUT_DIR/cosign.pub" "$OUT_FILE"

# --- success message (no key material) ----------------------------------------

cat <<MSG
done.

  encrypted Secret : $OUT_FILE
  public key       : $OUT_DIR/cosign.pub  (commit this)

next steps:
  1. verify decrypt round-trip:
       sops --decrypt $OUT_FILE | head -10
  2. apply to cluster (if that's your delivery path):
       sops exec-file $OUT_FILE 'kubectl apply -f {}'
  3. for Codeberg Actions, mirror the two fields into repo secrets:
       sops --decrypt $OUT_FILE | yq -r '.stringData["cosign.key"]'      → COSIGN_PRIVATE_KEY
       sops --decrypt $OUT_FILE | yq -r '.stringData["cosign.password"]' → COSIGN_PASSWORD
     (do this in a private terminal; do not redirect to disk.)
MSG
