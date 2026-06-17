#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
# Seal the three pipeline secrets into SealedSecrets for op1st-pipelines.
#
# Prompts for each value with no echo and no shell history; writes ONLY the
# sealed (safe-to-commit) YAML under tekton/secrets/; shreds plaintext. Run from
# the repo root with `oc` logged in to nostromo and kubeseal installed.
#
#   bash scripts/seal-secrets.sh
set -euo pipefail

NS=op1st-pipelines
CTRL_NS=sealed-secrets
CTRL_NAME=sealed-secrets-controller
OUT=tekton/secrets
mkdir -p "$OUT"

TMP="$(mktemp -d)"
trap 'find "$TMP" -type f -exec shred -u {} + 2>/dev/null; rm -rf "$TMP"' EXIT

seal() {
  local secret="$1" key="$2" prompt="$3"
  local f="$TMP/$secret"
  printf '%s' "" > "$f"
  read -rs -p "$prompt: " val; echo
  if [ -z "$val" ]; then echo "  ! empty value, skipping $secret" >&2; return 1; fi
  printf '%s' "$val" > "$f"
  unset val
  oc create secret generic "$secret" -n "$NS" \
       --from-file="$key=$f" --dry-run=client -o yaml \
  | kubeseal --controller-namespace "$CTRL_NS" --controller-name "$CTRL_NAME" \
       --format yaml > "$OUT/$secret.sealed.yaml"
  echo "  -> wrote $OUT/$secret.sealed.yaml"
}

echo "Sealing into namespace $NS via $CTRL_NS/$CTRL_NAME"
seal forgejo-mcp-forgejo-read  token   "Forgejo READ token (forgejo-mcp-pipeline-read)"
seal forgejo-mcp-forgejo-write token   "Forgejo WRITE token (forgejo-mcp-pipeline-write)"
seal forgejo-mcp-pipeline-agent api-key "Claude API key (sk-ant-...)"

echo
echo "Done. Sealed files:"
ls -1 "$OUT"/*.sealed.yaml
echo "Plaintext shredded. Review the sealed YAML, then apply with: kubectl apply -f $OUT/"
