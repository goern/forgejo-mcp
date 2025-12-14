# Wiki Support Status

**Status**: Pending upstream dependency

## Why Wiki Support Is Not Yet Available

Wiki tools (list, get, create, update, delete wiki pages) are planned but currently blocked by a missing dependency in the [forgejo-sdk](https://codeberg.org/mvdkleijn/forgejo-sdk).

The SDK (v2.0.0-v2.2.0) does not yet include wiki API methods. Rather than implementing a workaround with raw HTTP calls, we prefer to:

1. Contribute the wiki methods upstream to forgejo-sdk
2. Integrate them cleanly into forgejo-mcp once released

This approach ensures better maintainability and consistency with other operations.

## Tracking

- **Feature request**: See [wiki-support.md](./wiki-support.md) for the full implementation plan
- **Upstream contribution**: Pending PR to forgejo-sdk

## Workaround

If you need wiki functionality now, you can use the Forgejo API directly:

- `GET /api/v1/repos/{owner}/{repo}/wiki/pages`
- `GET /api/v1/repos/{owner}/{repo}/wiki/page/{pageName}`
- `POST /api/v1/repos/{owner}/{repo}/wiki/new`
- `PATCH /api/v1/repos/{owner}/{repo}/wiki/page/{pageName}`
- `DELETE /api/v1/repos/{owner}/{repo}/wiki/page/{pageName}`
