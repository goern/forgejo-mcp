## Releasing

Releases are cut by an OpenShift Job, never from your machine. The Job clones
**the release remote's main from the forge** — `just preflight` prints which
remote that is — and never sees your working tree: an unpushed commit is not in
the release, and a dirty tree is harmless.

- `just preflight` — every precondition, read-only. Run it first; `just release`
  runs it anyway and stops if it fails.
- `just release` — cut one. `just release-dry` surveys without cutting.
- Commit messages must be conventional (`feat:`, `fix:`, `chore:`, …). They are
  the only input to the version number and the changelog, so a release that
  produced nothing usually means nothing releasable was committed.

**`just release` returning is not the release finishing.** The Job creates the
tag and the Release object, then exits. If the repo has tag-triggered CI that
builds and attaches assets — binaries, SBOMs, signatures — that runs afterwards
and takes minutes longer. A release inspected in that window looks like it is
missing assets, and it is not: it is unfinished. Wait for the tag pipeline to
report success before judging anything absent.

`just --list` for the rest. Do not run semantic-release locally.
