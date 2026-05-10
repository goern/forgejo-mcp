---
name: codeberg-issue-triage
description: Triage a single issue on Codeberg (or any Forgejo/Gitea-compatible host). Use this skill whenever the user wants to assess, review, or triage a Codeberg issue — including when they paste a codeberg.org issue URL, say "look at issue 42 on owner/repo", "what should we do with this", "review this codeberg issue", or similar. Trigger even when the word "triage" isn't used: any request to read a Codeberg issue and recommend labels, a response, or what to do next falls under this skill. Reads the issue and its comments through the codeberg-mcp connector, then returns an assessment, proposed labels, a proposed English comment, and a two-part verdict (classification + action) for the user to apply manually.
---

# Codeberg Issue Triage

Help the user triage a single issue on Codeberg by reading it through the codeberg-mcp connector and returning a structured proposal: assessment, labels, comment, and verdict. The user reviews and applies things manually — this skill never writes to the repo.

## Inputs

You need two things:

- **repo_slug** — accepted as `owner/repo`, `org/repo`, or a full URL like `https://codeberg.org/owner/repo` or `https://codeberg.org/owner/repo/issues/42`
- **issue_id** — an integer

If you already have both (e.g. the user pasted a full issue URL), don't ask — just proceed. Otherwise, ask once, briefly, for whichever is missing.

Normalize URLs before calling tools:

- Strip the scheme and host (`https://codeberg.org/`).
- If the path contains `/issues/<n>`, extract `<n>` as the issue_id and use the segment before `/issues/` as the repo_slug.
- Drop trailing slashes, query strings, and fragments.
- The final repo_slug should be exactly `owner/repo`.

## Process

### 1. Fetch the issue and its context

Call the codeberg-mcp tools available in this session. You typically need three things:

- The issue itself — title, body, state, author, created/updated time, existing labels, assignees.
- The issue's comments — the conversation may have already converged on something, and ignoring it leads to embarrassing proposals.
- The repository's available labels — so proposed labels reuse existing names exactly.

Tool names vary by connector version. Look for something shaped like `get_issue`, `list_issue_comments`, `list_labels` (or close variants). If no codeberg-mcp tools are exposed in this session, say so and stop — do not fall back to web scraping or guessing.

If the issue is already closed, still triage it, but flag that in the assessment and let the verdict reflect it (often the work is just confirming the close reason was correct, or noting that no further action is needed).

### 2. Assess the issue

Read carefully and form a clear picture:

- What problem or request is *actually* being raised — versus what the title literally says.
- Whether it's reproducible, well-scoped, or vague.
- Whether the comment thread has already resolved or clarified things.
- Whether it duplicates another issue *you have evidence for* — never claim duplicate on a hunch.
- Tone signals: hostile, panicked, confused, polite-but-blocked. These don't change the verdict but they shape the comment.

Be honest. If the issue is poorly written or the reporter is frustrated, say so plainly without dismissing the underlying problem.

### 3. Propose labels

Strongly prefer labels that already exist on the repo — reuse the names exactly, including any prefix conventions like `kind/bug`, `area/api`, or `priority/high`. Codeberg labels are case-sensitive.

Only propose a *new* label when the existing set genuinely doesn't cover something the maintainers will want to filter on later. Mark new labels as `(new)` so the user knows they'd need to create them.

Cap proposals at around 4 labels. Triage labels are for sorting and prioritization, not for tagging every conceivable attribute.

### 4. Propose a comment

Always write the comment in **English**, regardless of the issue's language. If the issue is in German or another language, the user will adapt or translate manually — the skill's job is to give them a well-structured English starting point.

Match the comment to the verdict:

- `needs-info` → ask the specific missing questions, one per line, easy to answer.
- `duplicate` → link to the duplicate (only if you actually found one) and briefly explain.
- `wontfix` / `invalid` → explain why kindly; suggest an alternative if there is one.
- `valid + accept` → confirm receipt, summarize what's understood, set expectations for next step.
- `valid + needs-discussion` → flag the open design questions; don't ping specific people (the user decides who).

Keep it short — usually 2–6 sentences. Skip filler like "Thanks for the issue!" — be direct and useful.

### 5. Verdict

The verdict has two facets, both required.

**Classification** — what kind of issue this is:

- `valid` — real, actionable, well-scoped.
- `duplicate` — actually duplicates another issue (cite it).
- `needs-info` — cannot act without more from the reporter.
- `wontfix` — valid but out of scope or against project direction.
- `invalid` — not a real issue (user error, off-topic, spam).

**Action** — what should happen next:

- `accept` — take it on, label and queue.
- `reject` — close it; the proposed comment serves as the close message.
- `defer` — leave open, no immediate action, revisit later.
- `needs-discussion` — leave open and escalate (maintainers, RFC, architecture council).

Common pairings: `valid + accept`, `duplicate + reject`, `needs-info + defer`, `wontfix + reject`, `invalid + reject`, `valid + needs-discussion`. Less common pairings (e.g. `valid + reject` for something correct-but-policy-blocked) are fine when the situation calls for them — explain in the rationale.

## Output format

Use exactly this structure. The user copies parts of this into Codeberg's UI manually, so each section needs to stand alone and be paste-ready.

```
## Issue: <title> (#<id>)
**Repo**: <owner/repo> • **State**: <open|closed> • **Author**: @<user> • **Opened**: <YYYY-MM-DD>

### Assessment
<2–4 sentences>

### Proposed labels
- `<label>` — <existing|new> — <one-line rationale>
- ...

### Proposed comment
> <english comment, blockquoted>

### Verdict
**Classification**: <valid | duplicate | needs-info | wontfix | invalid>
**Action**: <accept | reject | defer | needs-discussion>

<1–2 sentence rationale tying classification and action together>
```

No additional sections, no preamble, no closing remarks. If something important didn't fit (e.g. a strong duplicate suspicion you couldn't confirm), put it in the rationale.

## Things to avoid

- **Never call tools that modify the issue** (add labels, post comments, change state, assign people) even if such tools are exposed by the connector. This skill is read-only and proposal-only by design — the user explicitly wants to apply things manually.
- Don't speculate about duplicates without a specific issue to point to. "This might duplicate something" is worse than nothing.
- Don't mirror a hostile or panicked tone — stay neutral and concrete in the proposed comment.
- Don't propose a label you can't justify in one short clause. If you can't say why it helps, drop it.
- Don't write the comment in the issue's language — always English, per the design of this skill.

## Example output

```
## Issue: Crash on startup when XDG_CONFIG_HOME is unset (#142)
**Repo**: example-org/cool-tool • **State**: open • **Author**: @reporter • **Opened**: 2026-04-18

### Assessment
A reproducible startup crash when `XDG_CONFIG_HOME` is unset; the reporter included a stack trace pointing at `config.load()`. The two follow-up comments confirm the same crash on a clean system. Scope is small and the fix is likely a default-path fallback.

### Proposed labels
- `kind/bug` — existing — clear, reproducible defect
- `area/config` — existing — fault is in the config loader
- `good-first-issue` — existing — small, well-scoped, has a stack trace

### Proposed comment
> Thanks for the trace — reproduced. The loader assumes `XDG_CONFIG_HOME` is always set; we should fall back to `~/.config` per the XDG spec. Marking as a good first issue; happy to mentor a PR.

### Verdict
**Classification**: valid
**Action**: accept

Reproducible defect with a clear root cause and a low-risk fix; no blockers to accepting and queueing it.
```
