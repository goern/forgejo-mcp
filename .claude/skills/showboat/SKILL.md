---
name: showboat
description: Convention for building a reproducible demo artifact co-located with each implementation spec, so acceptance has something concrete to read. The author builds the demo against a running instance during the implementation PR; the reviewer reads it during acceptance without re-executing. Triggers on "build demo", "acceptance demo", "showboat", or any spec-linked PR that needs a proof-of-work artifact.
---

<essential_principles>

## Scope

This skill encodes **forgejo-mcp-local conventions** for Showboat. It is
self-contained — it does not depend on any external recipe skill.

### What Showboat Is

Showboat is a CLI (`uvx showboat`) that builds Markdown "demo documents" that
interleave narrative, executable code blocks, and captured output. A demo
produced by Showboat is simultaneously a readable story and a reproducible
script. Think *lab notebook snapshot*: the author ran the commands, Showboat
captured their output verbatim, and the resulting file is committed as proof.

Core commands — **always run `showboat --help` for the authoritative
reference**, do not memorize flags:

- `showboat init <file> <title>` — start a new demo
- `showboat note <file> [text]` — append narrative text
- `showboat exec <file> <lang> [code]` — run a command and capture its output
- `showboat image <file> <path>` — embed an image
- `showboat pop <file>` — remove the last entry (use when a command errored)

### 1. One Demo Per Spec, Co-Located

Every implementation PR that closes a spec-linked issue commits a demo file
co-located with the spec it proves. The host skill specifies the canonical
demo path — follow that convention.

### 2. Author Builds, Reviewer Reads

The author constructs the demo live against a running instance, captures real
output, commits the Markdown. **The reviewer does not re-execute.** They read
the file like a lab notebook during acceptance. Two reasons:

- **Environment independence** — the reviewer needs no runtime, no deploy
  step, no showboat install. They read markdown in the PR diff.
- **Determinism** — structured output typically contains timestamps and
  generated IDs that drift across runs. `showboat verify` would diff-fail
  on every re-run. We do not run verify on the critical path today.

### 3. Trust the Author at Commit Time

No automation re-executes the demo. The contract: **the author ran the
commands and committed the real captured output**. Hand-writing an output
block to hide a failure is a skill violation.

**Retrofit exception:** A retrofitted demo may commit placeholder output
blocks (`# TODO: re-run against live instance`) where real output is stale
or unavailable. The placeholder is explicit — it documents a gap, not
hides a failure.

### 4. Demo Through the Project's Structured Surface

Prefer subcommands that emit structured output (JSONL or similar) over raw
database queries, raw runtime RPCs, or arbitrary shell.

If you cannot demonstrate an acceptance criterion through the project's
instrumented CLI, that is a **missing subcommand in the project's
observability contract** — not a showboat problem. Stop, add the subcommand
in the same PR, then resume the demo.

### 5. One Section Per Acceptance Criterion (non-anchored specs)

For non-anchored specs the narrative maps 1:1 to the spec's acceptance list:

- One `showboat note` anchoring the criterion (heading + paraphrase)
- One or more `showboat exec` blocks producing evidence
- No criterion without evidence; no stray evidence without an anchor

For **anchored specs** (with `<!-- demos-anchored: true -->`), the 1:1
mapping is by `#### Scenario:` heading instead — see `anchored-mode.md`.

</essential_principles>

<mode_dispatch>

## Mode Dispatch

Read the sibling `spec.md` first. Then choose:

| Condition | Mode | Read |
|-----------|------|------|
| `spec.md` has `<!-- demos-anchored: true -->` before its first H2, demo doesn't exist | **New anchored** | `anchored-mode.md` |
| `spec.md` has `<!-- demos-anchored: true -->`, sibling demo exists in old shape | **Retrofit** | `retrofit-mode.md` (which depends on `anchored-mode.md`) |
| Caller explicitly says "retrofit" | **Retrofit** | `retrofit-mode.md` |
| `spec.md` has no anchor marker | **New non-anchored** | this file (workflow §New Demo below) |

Before authoring in any mode, scan `authoring-smells.md` — the seven
antipatterns there apply universally.

</mode_dispatch>

<intake>

Required inputs, supplied by the host skill:

- **Spec path** — the spec file whose acceptance criteria the demo will prove
- **Demo path** — the co-located output path convention (e.g. `<spec-dir>/<slug>.demo.md`)
- **Running instance** — the branch under review, deployed and reachable
- **Project CLI** — the instrumented subcommand surface the demo should use

For retrofit mode, the running instance is optional (placeholder output is
acceptable). For new anchored or non-anchored demos, it is required.

If any required input is missing, ask before proceeding.

</intake>

<workflow>

### Portable Binary Reference (path portability)

**Never embed absolute paths to a local build in evidence commands.** Use
the env-var pattern and document the setup at the top of the demo:

````markdown
## Replay setup

```bash
export FORGEJO_MCP_BIN="${FORGEJO_MCP_BIN:-forgejo-mcp}"
# Point at a local build: export FORGEJO_MCP_BIN=./forgejo-mcp
```
````

Then reference the binary as `${FORGEJO_MCP_BIN}` in all evidence commands.
For arbitrary repo-rooted scripts, use `${SHOWBOAT_REPO:-$(git rev-parse --show-toplevel)}`.
See `authoring-smells.md` smells #1 and #7.

### Scaffold Provenance Placeholders

When starting a new demo, include these authoring-provenance markers
immediately after the title line so the file is traceable before real
output is captured:

```markdown
*Captured: <ISO date> via Showboat <ver>*
<!-- captured-for: PR #<n> -->
<!-- captured-at: <ISO date> -->
<!-- captured-against: <git-sha-or-branch> -->
```

Replace all four with real values before committing. The machine-readable
comments (`captured-for`, `captured-at`, `captured-against`) let future
tooling correlate demo files with the PR and exact commit that produced them
without parsing prose.

### New Demo (Non-Anchored Spec)

1. **Discover.** Run `showboat --help`. This skill deliberately does not restate it.
2. **Deploy.** Ensure the current branch is live on a local instance.
3. **Init.** `showboat init <demo-path> "<spec title>"`. Add the scaffold provenance placeholders above.
4. **Anchor.** Add a `## Replay setup` block, then `showboat note` lines linking the spec path, the issue, and the PR.
5. **Baseline.** One `showboat exec` capturing the starting state via the project's status/inspect subcommand.
6. **Walk ACs.** For each acceptance criterion: a `note` with the AC heading, then `exec` block(s) triggering the behavior, then an `exec` capturing evidence via a structured-output subcommand.
7. **Commit.** Add the demo file to the PR diff. Reference its path in the PR body per the host skill's template.

### New Demo (Anchored Spec)

See `anchored-mode.md` for the full procedure, slug derivation, block shape, and evidence kinds.

### Retrofit Existing Demo

See `retrofit-mode.md` for the full procedure. Manually review anchor/scenario parity after writing (no automated checker is wired up in this repo yet).

</workflow>

<success_criteria>

**Non-anchored demo:**
- [ ] File exists at the demo path specified by the host skill
- [ ] File opens with a link back to the spec and the issue/PR numbers
- [ ] Every acceptance criterion in the spec has a matching `note` + `exec` section
- [ ] Evidence commands use the project's structured-output subcommands wherever possible
- [ ] Output blocks are real captured output (no hand-edited content)
- [ ] PR body links to the demo file

**Anchored demo (opted-in spec):**
- [ ] `<!-- demos-anchored: true -->` present in `spec.md`
- [ ] Every `#### Scenario:` in `spec.md` has a matching proof block in `demo.md`
- [ ] Each proof block: machine anchor → human anchor → quoted scenario → evidence block
- [ ] Machine and human anchor share the exact same slug
- [ ] Quoted scenario text matches `spec.md` verbatim (whitespace-normalized)
- [ ] At least one evidence block per proof (fenced code or `evidence-kind` marker)
- [ ] Anchor/scenario parity manually verified before committing (no automated checker in this repo yet)
- [ ] No absolute `PATH=/Users/…` in evidence commands (use `${FORGEJO_MCP_BIN:-forgejo-mcp}`)
- [ ] No `## AC<n>` H2 headings shadowing a quoted `#### Scenario:` H4
- [ ] No AC numbering gaps from mid-sequence scenario insertion

</success_criteria>

<references_index>

| Reference | Purpose |
|-----------|---------|
| `anchored-mode.md` | Anchored demo authoring — slug, block shape, evidence kinds |
| `retrofit-mode.md` | Retrofit procedure for existing demos under newly-anchored specs |
| `authoring-smells.md` | Seven antipatterns; scan before authoring in any mode |
| `showboat --help` | Authoritative CLI reference — run it, do not memorize flags |
| Host project's observability contract | The structured-output surface demos ride on |
| Host skill's implementation workflow | Where demo construction is sequenced |

</references_index>
