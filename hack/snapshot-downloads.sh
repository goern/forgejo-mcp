#!/bin/sh
# Snapshot per-asset release download counts into an append-only JSONL time series.
#
# The Forgejo API exposes a *cumulative* download_count per release asset but keeps
# no history. We snapshot it daily; git becomes the time-series database. Each run
# appends one line per (date, tag, asset). Re-running on the same day replaces that
# day's rows (idempotent), so a retried CI job never double-counts.
#
# POSIX sh (no bashisms): the CI runner's pod image may not ship bash.
#
# Usage: hack/snapshot-downloads.sh [owner/repo] [output.jsonl]
set -eu

REPO="${1:-goern/forgejo-mcp}"
OUT="${2:-docs/downloads/downloads.jsonl}"
API="${FORGEJO_API:-https://codeberg.org/api/v1}"
TODAY="$(date -u +%F)"

mkdir -p "$(dirname "$OUT")"

# Paginate releases until a page comes back empty, flattening assets to one row
# each. download_count is the cumulative counter we are sampling at $TODAY.
snapshot="$(mktemp)"
trap 'rm -f "$snapshot"' EXIT INT TERM

page=1
while : ; do
  body="$(curl -fsS "$API/repos/$REPO/releases?limit=50&page=$page")"
  count="$(printf '%s' "$body" | jq 'length')"
  [ "$count" -eq 0 ] && break
  printf '%s' "$body" | jq -c --arg date "$TODAY" '
    .[] as $rel
    | $rel.assets[]
    | {date:$date, tag:$rel.tag_name, published:$rel.published_at,
       asset:.name, size:.size, downloads:.download_count}
  ' >> "$snapshot"
  page=$((page + 1))
done

if [ ! -s "$snapshot" ]; then
  echo "snapshot-downloads: no assets returned for $REPO — aborting (kept existing $OUT)" >&2
  exit 1
fi

# Idempotent append: drop any rows already recorded for $TODAY, then add fresh ones.
if [ -f "$OUT" ]; then
  grep -v "\"date\":\"$TODAY\"" "$OUT" > "$OUT.tmp" || true
  mv "$OUT.tmp" "$OUT"
fi
cat "$snapshot" >> "$OUT"

echo "snapshot-downloads: recorded $(wc -l < "$snapshot") asset rows for $TODAY into $OUT"
