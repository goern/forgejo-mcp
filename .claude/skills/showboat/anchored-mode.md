# Anchored Demo Mode

> **Authoring model:** Anchored demos are **hand-authored Markdown**, not
> produced by `uvx showboat` CLI commands. You write the anchor blocks,
> quoted scenario text, and evidence blocks directly in the file. The
> `showboat exec` / `showboat note` CLI is for non-anchored demos only.

When a `spec.md` begins with `<!-- demos-anchored: true -->` before its
first H2 heading, the spec has opted in to **anchor enforcement**. In this
mode every proof block in the sibling demo MUST carry a machine anchor and
a human anchor that bind it to a specific `#### Scenario:` in the spec.

Use this mode when:
- Creating a new demo for an opted-in spec.
- Retrofitting an existing demo that has the marker (see `retrofit-mode.md`).

Do NOT use this mode for specs without the marker — the AC-based workflow
(see SKILL.md §5) remains correct for non-opted-in specs.

## Detecting Opt-In

Before starting, read the sibling `spec.md`. If the first non-blank line or
any line before the first `##` heading is exactly `<!-- demos-anchored: true -->`,
anchored mode is active.

## Slug Derivation

The **slug** for a scenario is derived from its `#### Scenario:` heading text
by GitHub-style slugification:

1. Lowercase the heading text.
2. Collapse every run of non-alphanumeric characters to a single `-`.
3. Trim leading and trailing `-`.

Example: `#### Scenario: Command opens the passive widget`
→ slug: `command-opens-the-passive-widget`

Both the machine and human anchor share the exact same slug. Never author
them independently — derive from the heading, apply to both.

## Anchored Proof Block Shape

Each scenario proof consists of four consecutive elements:

```
<!-- spec-scenario: <capability>#<slug> -->
**Proves:** [spec.md → Scenario: <Scenario Heading Text>](./spec.md#scenario-<slug>)

#### Scenario: <Scenario Heading Text>
- **WHEN** <copy WHEN bullet verbatim from spec.md>
- **THEN** <copy THEN bullet verbatim from spec.md>
- **AND** <copy AND bullet verbatim from spec.md, if present>

<!-- evidence-kind: <kind> -->
```<lang>
<command and/or output>
```
```

Rules:
1. **Machine anchor** — HTML comment on its own line. `<capability>` is the
   spec's directory name (e.g. `merge-pull-request`). `<slug>` is derived as above.
2. **Human anchor** — `**Proves:**` link on the very next line. The `#scenario-<slug>`
   fragment matches GitHub's auto-generated anchor for the heading.
3. **Quoted scenario** — the `#### Scenario:` heading and all WHEN/THEN/AND
   bullets copied verbatim from `spec.md`. Do not paraphrase. Do not omit.
   This is mandatory; it makes the demo self-readable without opening `spec.md`.
4. **Evidence block** — at least one fenced code block (any language) OR an
   `<!-- evidence-kind: <kind> -->` marker followed by block content. One block
   per scenario minimum. The checker validates presence, not content.

> Before authoring: see `authoring-smells.md` for concrete antipatterns
> that break replayability and readability.

## Evidence Kinds

Four canonical kinds. Use the `<!-- evidence-kind: <kind> -->` marker when the
evidence is not a plain shell capture:

| Kind | When to use | Example |
|------|-------------|---------|
| `showboat-cli` | Captured `forgejo-mcp` CLI command + stdout. Default for observable behaviors. | `$ forgejo-mcp --help` + output |
| `test-invocation` | Single targeted test command + its output. Use when the scenario maps 1:1 to a test function. | `$ go test ./pkg/forgejo/ -run TestClient_AuthorizationHeader -v` |
| `unit-test-output` | Captured output of a multi-test run; include a comment naming which test(s) inside prove the scenario. Use when 1:1 mapping is too expensive. | `$ go test ./...` + note: "see `TestExtractToken/bare_token_rejected` assertion" |
| `external-artifact` | Link to a file or run elsewhere in the repo (e.g. fixture, CI run URL). | `See demos/multi-tenant-http.md` |

**Prefer `test-invocation` when a 1:1 test exists.** Retrofit mode surfaces
candidates automatically (see `retrofit-mode.md`). Fall back to
`unit-test-output` when splitting the test is too expensive; record the
decision inline.

## Reading Order (Asymmetric)

The convention is shaped by who reads what first:

- **Human reader** enters from `demo.md` first. The concrete output is the
  legible artifact; they follow the human anchor link to `spec.md` only if
  they want the formal requirement. The quoted scenario heading and WHEN/THEN
  bullets inside each proof block are therefore **mandatory** — they make the
  demo self-readable without navigating to `spec.md`.

- **AI agent** enters from `spec.md` first (it is the norm for acceptance
  checks). Finding the proof: `demo.md` is always the sibling file. Machine
  anchors `<!-- spec-scenario: <cap>#<slug> -->` are grep-findable.

**Optional back-link from `spec.md`:** An opted-in `spec.md` MAY (but is not
required to) place a back-link above each scenario:

```
<!-- proven-in: ./demo.md#scenario-<slug> -->
#### Scenario: <heading>
```

The checker verifies the back-link when present but does NOT require it.
Only add it when the agent/tooling use case explicitly needs the spec→demo
navigation to be machine-readable without grep.

## Procedure: New Anchored Demo

1. **Detect opt-in.** Confirm `<!-- demos-anchored: true -->` is present in `spec.md`.
2. **Extract scenarios.** Read every `#### Scenario:` heading and WHEN/THEN/AND bullets from `spec.md`. Compute slug for each.
3. **Deploy.** Ensure the current branch is live.
4. **Add `## Replay setup` block.** Document `FORGEJO_MCP_BIN` at the top of the demo before the first anchor (see SKILL.md "Portable Binary Reference").
5. **For each scenario in spec order:**
   a. Write the machine anchor and human anchor.
   b. Copy the scenario heading and WHEN/THEN/AND bullets verbatim.
   c. Run the evidence command and capture output, OR grep for a test candidate.
   d. Write the evidence block with the appropriate `<!-- evidence-kind: ... -->` marker.
6. **Commit.** Run `just check-demos` to confirm the spec passes before committing.
