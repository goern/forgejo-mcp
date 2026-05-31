# Release download analytics

Tracks how often each release asset is downloaded from Codeberg, over time.

## Why snapshots

The Forgejo API exposes a **cumulative** `download_count` per release asset
(`GET /api/v1/repos/{owner}/{repo}/releases`) but keeps **no history**. To see
trends you must sample the counter periodically and store it yourself. We sample
daily and append to `downloads.jsonl` — git is the time-series database.

## Pieces

| File | Role |
|------|------|
| `hack/snapshot-downloads.sh` | Pulls the API, flattens assets, appends one JSONL row per `(date, tag, asset)`. Idempotent per day. |
| `.forgejo/workflows/track-downloads.yml` | Daily cron (06:17 UTC) that runs the script and commits the new rows. |
| `docs/downloads/downloads.jsonl` | The append-only time series. |
| `docs/downloads/index.html` | Self-contained Chart.js dashboard. Fetches the JSONL, classifies assets + releases client-side. |

## Run it locally

```bash
hack/snapshot-downloads.sh                 # append today's snapshot
python3 -m http.server -d docs/downloads   # then open http://localhost:8000
```

## CI requirements

This is the **only** Forgejo Actions workflow in the repo (CI/CD is otherwise
Tekton on the op1st cluster). It needs:

1. **Forgejo Actions enabled** for the repo (Settings → Actions → Enable).
2. A repo/org secret **`DOWNLOAD_TRACKER_TOKEN`** — a token with `write:repository`
   used to push the daily commit back to the default branch.

## Charts

- **Per release × platform** — stacked bar of the latest snapshot. The binary
  `tar.gz` assets are the real usage signal; `.mcpb` bundles and metadata
  (checksums/sig/SBOM) are mostly automated tooling.
- **By release type** — each tag classified by its semver bump vs the previous
  version (major = breaking, minor = feat, patch = fix), summing binary downloads.
- **Binary downloads over time** — grows as daily snapshots accumulate.
