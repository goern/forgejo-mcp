# Authoring Smells
<!-- authoring smells — antipattern index -->

Seven concrete antipatterns surfaced from demo review. Each has a
one-line do/don't. All apply to both new and retrofit demos.

## 1. Absolute build paths (replayability)

**DON'T:** `PATH=/Users/alice/src/forgejo-mcp--20260425:$PATH forgejo-mcp …`

**DO:** Use `${FORGEJO_MCP_BIN:-forgejo-mcp}` and add a `## Replay setup` block
at the top of the demo documenting how to point `FORGEJO_MCP_BIN` at a local
build. Absolute paths from the author's worktree break replayability for
every other developer and reviewer.

## 2. Doubled H2 + H4 scenario headings (heading dedup)

**DON'T:**
```markdown
## AC1 — Config, state, and cache have distinct homes

#### Scenario: Config, state, and cache have distinct homes
- **WHEN** …
```

**DO:** Drop the `## AC<n> —` H2 entirely. The `#### Scenario:` H4 inside
the anchor block is the canonical proof heading. The H2 was scaffolding for
the old AC-based format; in anchored mode it creates a redundant, stale
navigation layer.

## 3. AC numbering with insertion gaps (false continuity)

**DON'T:** `## AC1 … ## AC3 … ## AC5` after inserting new scenarios between
existing ones, leaving gaps in the sequence.

**DO:** Drop AC numbering entirely on retrofit. Use the scenario heading as
the sole identifier. Numbered headings create false continuity when
sequences evolve; the spec itself is the authoritative ordering.

## 4. Stale `[openspec/changes/<old-name>]` provenance links (link rot)

**DON'T:** Keep a full multi-field header block (`**Spec:** … **Change:** …
**PR:** …`) pointing at a change directory that has since been merged or
superseded.

**DO:** Replace the block with a single provenance one-liner:

```
*Originally captured for `<old-cap>` / PR #N; merged into `<new-cap>` on YYYY-MM-DD.*
```

Do not try to keep multi-field provenance alive after consolidation — it
rots faster than one line of prose.

## 5. Heterogeneous block shape (inconsistent evidence shape)

**DON'T:** Mix terse `cmd + output` blocks in some scenarios with rich
`lead-in prose + cmd + output + contrast note` blocks in others within the
same demo.

**DO:** Pick one shape per demo and apply it consistently:
- **Terse:** machine anchor → human anchor → quoted scenario → `cmd` block → `output` block
- **Rich:** machine anchor → human anchor → quoted scenario → lead-in sentence → `cmd` block → `output` block → contrast / explanation

Heterogeneous shape makes the demo harder to scan and signals that the
demo grew by accretion rather than authoring intent.

## 6. Test-name-only proof fitness (proof opacity)

**DON'T:** Use `test-invocation` evidence where the test function name does
not directly map to the scenario's THEN clause, leaving the reader unable
to verify the mapping without reading the test source.

**DO:** When a test name doesn't clearly map to THEN, downgrade to
`unit-test-output` and add an assertion-naming comment:

```markdown
<!-- evidence-kind: unit-test-output -->
<!-- proves: asserts that a bare token is rejected, not forged into an identity — see TestExtractToken/bare_token_rejected -->
```bash
go test ./operation/ -run TestExtractToken
```
```

This is the D5 design decision: proof fitness must be legible at demo-read
time, not only at test-source-read time.

## 7. Absolute repo paths in evidence commands (path portability)

**DON'T:** Hard-code the worktree directory in paths inside evidence blocks:

```bash
$ /Users/alice/src/forgejo-mcp--20260425/scripts/run_fixtures.sh
```

**DO:** Use repo-root-relative paths (e.g. `scripts/run_fixtures.sh`,
run from repo root) or `${SHOWBOAT_REPO:-$(git rev-parse --show-toplevel)}`
for scripts that require an absolute prefix. Document the variable in the
`## Replay setup` block and add `export SHOWBOAT_REPO="$(pwd)"` to the
setup instructions.

This smell differs from smell #1 (binary path): it applies to *any file path*
inside a command that is baked from the author's worktree location — not just
`PATH=` entries. Both must be replaced on retrofit.
