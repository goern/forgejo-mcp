#!/usr/bin/env bash
# E2E test for issue-attachment MCP tools (v2.19.0-alpha.1+).
#
# Validates that ./forgejo-mcp --cli can:
#   1. create an issue
#   2. upload an attachment
#   3. list attachments and see the one we just uploaded
#   4. fetch single-attachment metadata, with browser_download_url present
#   5. fetch the URL with the configured token and recover the original bytes
#
# Cleans up (deletes attachment, closes issue) on success or failure.
#
# Usage:
#   FORGEJO_URL=https://codeberg.org \
#   FORGEJO_ACCESS_TOKEN=... \
#   OWNER=goern REPO=forgejo-mcp \
#     ./test/e2e/attachments.sh
#
# Requires: jq, curl, base64, ./forgejo-mcp built (run `make build` first).

set -euo pipefail

: "${FORGEJO_URL:?FORGEJO_URL required (e.g. https://codeberg.org)}"
: "${FORGEJO_ACCESS_TOKEN:?FORGEJO_ACCESS_TOKEN required}"
OWNER="${OWNER:-goern}"
REPO="${REPO:-forgejo-mcp}"
BIN="${BIN:-./forgejo-mcp}"

[[ -x "$BIN" ]] || { echo "binary not found at $BIN — run 'make build' first" >&2; exit 1; }
command -v jq >/dev/null   || { echo "jq required" >&2; exit 1; }
command -v curl >/dev/null || { echo "curl required" >&2; exit 1; }

# --- helpers ----------------------------------------------------------------

# Run a forgejo-mcp tool and unwrap the {"Result":...} envelope.
# Tools wrap output as [{"type":"text","text":"{\"Result\":...}"}].
mcp() {
    local tool="$1" args="$2"
    "$BIN" --cli "$tool" --args "$args" 2>/dev/null \
        | jq -r '.[] | select(.type=="text") | .text' \
        | jq '.Result // .'
}

# Status line on a green background; failure on red.
ok()   { printf '\033[32m✓ %s\033[0m\n' "$*"; }
fail() { printf '\033[31m✗ %s\033[0m\n' "$*" >&2; exit 1; }
step() { printf '\n\033[1m▶ %s\033[0m\n' "$*"; }

# --- state for cleanup ------------------------------------------------------

ISSUE_INDEX=""
ATTACH_ID=""

cleanup() {
    local rc=$?
    set +e
    if [[ -n "$ATTACH_ID" && -n "$ISSUE_INDEX" ]]; then
        echo "[cleanup] delete attachment $ATTACH_ID"
        mcp delete_issue_attachment \
            "$(jq -nc --arg o "$OWNER" --arg r "$REPO" --argjson i "$ISSUE_INDEX" --argjson a "$ATTACH_ID" \
                '{owner:$o, repo:$r, index:$i, attachment_id:$a}')" >/dev/null
    fi
    if [[ -n "$ISSUE_INDEX" ]]; then
        echo "[cleanup] close issue #$ISSUE_INDEX"
        mcp issue_state_change \
            "$(jq -nc --arg o "$OWNER" --arg r "$REPO" --argjson i "$ISSUE_INDEX" \
                '{owner:$o, repo:$r, index:$i, state:"closed"}')" >/dev/null
    fi
    exit "$rc"
}
trap cleanup EXIT

# --- test payload -----------------------------------------------------------

PAYLOAD="forgejo-mcp e2e attachment test $(date -u +%s) $RANDOM"
PAYLOAD_FILE="$(mktemp)"
printf '%s' "$PAYLOAD" > "$PAYLOAD_FILE"
PAYLOAD_B64="$(base64 -w0 "$PAYLOAD_FILE")"
PAYLOAD_SHA="$(printf '%s' "$PAYLOAD" | sha256sum | cut -d' ' -f1)"

# --- 1. create issue --------------------------------------------------------

step "create issue"
ISSUE_JSON="$(mcp create_issue \
    "$(jq -nc --arg o "$OWNER" --arg r "$REPO" \
        '{owner:$o, repo:$r, title:"[e2e] attachment test", body:"transient — will be auto-closed"}')")"
ISSUE_INDEX="$(echo "$ISSUE_JSON" | jq '.number')"
[[ "$ISSUE_INDEX" =~ ^[0-9]+$ ]] || fail "create_issue did not return a numeric .number; got: $ISSUE_JSON"
ok "issue #$ISSUE_INDEX created"

# --- 2. upload attachment ---------------------------------------------------

step "upload attachment"
CREATE_JSON="$(mcp create_issue_attachment \
    "$(jq -nc --arg o "$OWNER" --arg r "$REPO" --argjson i "$ISSUE_INDEX" \
              --arg c "$PAYLOAD_B64" --arg fn "e2e-test.txt" --arg mt "text/plain" \
        '{owner:$o, repo:$r, index:$i, content:$c, filename:$fn, mime_type:$mt}')")"
ATTACH_ID="$(echo "$CREATE_JSON" | jq '.id')"
ATTACH_URL="$(echo "$CREATE_JSON" | jq -r '.browser_download_url')"
ATTACH_SIZE="$(echo "$CREATE_JSON" | jq '.size')"
[[ "$ATTACH_ID" =~ ^[0-9]+$ ]] || fail "create_issue_attachment did not return numeric .id; got: $CREATE_JSON"
[[ "$ATTACH_URL" == http*://* ]] || fail "create did not return browser_download_url; got: $ATTACH_URL"
[[ "$ATTACH_SIZE" -eq "${#PAYLOAD}" ]] || fail "uploaded size $ATTACH_SIZE != local size ${#PAYLOAD}"
ok "attachment $ATTACH_ID uploaded ($ATTACH_SIZE bytes)"
ok "browser_download_url = $ATTACH_URL"

# --- 3. list attachments ----------------------------------------------------

step "list attachments"
LIST_JSON="$(mcp list_issue_attachments \
    "$(jq -nc --arg o "$OWNER" --arg r "$REPO" --argjson i "$ISSUE_INDEX" \
        '{owner:$o, repo:$r, index:$i}')")"
LIST_LEN="$(echo "$LIST_JSON" | jq 'length')"
[[ "$LIST_LEN" -ge 1 ]] || fail "list returned $LIST_LEN entries; expected ≥ 1"
LIST_HAS_ID="$(echo "$LIST_JSON" | jq --argjson a "$ATTACH_ID" 'map(.id) | index($a) != null')"
[[ "$LIST_HAS_ID" == "true" ]] || fail "uploaded attachment $ATTACH_ID not in list: $LIST_JSON"
ok "list contains attachment $ATTACH_ID"

# --- 4. get single attachment ----------------------------------------------

step "get attachment metadata"
GET_JSON="$(mcp get_issue_attachment \
    "$(jq -nc --arg o "$OWNER" --arg r "$REPO" --argjson i "$ISSUE_INDEX" --argjson a "$ATTACH_ID" \
        '{owner:$o, repo:$r, index:$i, attachment_id:$a}')")"
GET_URL="$(echo "$GET_JSON" | jq -r '.browser_download_url')"
GET_NAME="$(echo "$GET_JSON" | jq -r '.name')"
[[ "$GET_URL" == "$ATTACH_URL" ]] || fail "get returned different URL: $GET_URL vs $ATTACH_URL"
[[ "$GET_NAME" == "e2e-test.txt" ]] || fail "get returned wrong name: $GET_NAME"
ok "get returns matching url + name"

# --- 5. download via browser_download_url + auth header --------------------

step "fetch browser_download_url with token (over-cap fall-through path)"
FETCHED="$(curl -sS -f -H "Authorization: token $FORGEJO_ACCESS_TOKEN" "$ATTACH_URL")"
FETCHED_SHA="$(printf '%s' "$FETCHED" | sha256sum | cut -d' ' -f1)"
[[ "$FETCHED_SHA" == "$PAYLOAD_SHA" ]] || fail "downloaded sha $FETCHED_SHA != uploaded sha $PAYLOAD_SHA"
ok "download bytes match (sha256=$PAYLOAD_SHA)"

# --- 6. download via MCP tool (under-cap inline path) ----------------------

step "download via MCP tool (under-cap → inline base64 blob)"
DL_RAW="$("$BIN" --cli download_issue_attachment \
    --args "$(jq -nc --arg o "$OWNER" --arg r "$REPO" --argjson i "$ISSUE_INDEX" --argjson a "$ATTACH_ID" \
        '{owner:$o, repo:$r, index:$i, attachment_id:$a}')" 2>/dev/null)"
DL_TEXT="$(echo "$DL_RAW" | jq -r '.[] | select(.type=="text") | .text')"
DL_INLINE="$(echo "$DL_TEXT" | jq '.inline')"
[[ "$DL_INLINE" == "true" ]] || fail "expected inline=true for ${#PAYLOAD}-byte payload; got: $DL_TEXT"
DL_BLOB="$(echo "$DL_RAW" | jq -r '.[] | select(.type=="resource") | .resource.blob')"
[[ -n "$DL_BLOB" && "$DL_BLOB" != "null" ]] || fail "no resource.blob in download response: $DL_RAW"
DL_BYTES="$(printf '%s' "$DL_BLOB" | base64 -d)"
DL_SHA="$(printf '%s' "$DL_BYTES" | sha256sum | cut -d' ' -f1)"
[[ "$DL_SHA" == "$PAYLOAD_SHA" ]] || fail "MCP-downloaded sha $DL_SHA != uploaded sha $PAYLOAD_SHA"
ok "MCP download bytes match (sha256=$PAYLOAD_SHA)"

echo
ok "E2E PASSED — issue #$ISSUE_INDEX, attachment $ATTACH_ID"
