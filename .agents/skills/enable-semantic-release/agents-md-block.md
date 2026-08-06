## Releasing

Releases are cut by an OpenShift Job, never from your machine. The Job clones
**origin/main from the forge** and never sees your working tree: an unpushed
commit is not in the release, and a dirty tree is harmless.

- `just preflight` — every precondition, read-only. Run it first; `just release`
  runs it anyway and stops if it fails.
- `just release` — cut one. `just release-dry` surveys without cutting.
- Commit messages must be conventional (`feat:`, `fix:`, `chore:`, …). They are
  the only input to the version number and the changelog, so a release that
  produced nothing usually means nothing releasable was committed.

`just --list` for the rest. Do not run semantic-release locally.
