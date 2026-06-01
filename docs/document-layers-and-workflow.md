# Document Layers & Development Workflow

How we structure product and architecture documents, and which format to reach
for when. The model is borrowed from the Spellkave `specifications/` knowledge
base; this document captures the layering and the decision rules.

## The core idea: a temporal axis

Every document answers a question about **time**. The mistake to avoid is mixing
tenses in one file — a "what we plan to do" sentence buried in a "what the
system is" document rots and misleads. So each layer owns exactly one tense.

| Layer | Tense | Question it answers | Mutability |
|---|---|---|---|
| **PRD** | Future — *intent* | What are we building, and for whom? | Mutable until realized |
| **ADR** | Future → Past — *decision* | Why did we choose this? | Body append-only; only `Status:` moves |
| **OpenSpec spec** | Present — *what is* | What does the system do today? | Mutable in place; git is the history |
| **OpenSpec change** | Future — *in flight* | What is this one unit of work? | Lives and dies with the change |

Rule of thumb: never write present-tense state ("the system currently does X")
in a PRD or ADR. That sentence belongs in an OpenSpec spec.

---

## PRD — Product Requirements Document

**Tense:** future. **Owns:** intent.

A PRD is product canon: mission, the problem, target users, product direction,
domain context, business rationale, and high-level success outcomes. It is
deliberately *not* a place for acceptance criteria or technical shape.

**Write a PRD when** you are defining or changing *what* the product is and *who*
it serves — the vision and scope, not the mechanism.

**A PRD explicitly does NOT define:**

- Feature-level behavior and acceptance criteria → OpenSpec change/spec
- Architectural decisions and implementation shape → ADR

Status lifecycle: `Draft → Review → Approved → Superseded`. Mutable until the
intent is realized.

---

## ADR — Architecture Decision Record

**Tense:** future when `Proposed`, past once landed. **Owns:** the decision and
its reasoning.

An ADR records a **decision at a point in time**: the choice made, the
alternatives rejected, and the *why*. It does not describe what the system is
today — that lives in the matching OpenSpec spec. The ADR is the record of *why*
the choice was made; the spec is *what it is now*.

**Write an ADR when the decision is one of:**

1. **Cross-cutting** — multiple capabilities will reference it.
2. **Load-bearing implementation shape** — gets baked in deeply.
3. **A binding architectural choice** with downstream effects.

Capability-local shape that touches only one feature does *not* need an ADR — it
stays in that capability's spec. When in doubt, stay out: an ADR is a permanent
record, cheaper to skip than to maintain wrongly.

### Status lifecycle

- **`Proposed`** — open decision, under discussion (in PR review).
- **`Active`** — accepted and in force. Flip from `Proposed` when it lands.
- **`Superseded by NNNN`** — retired by a later ADR; stays as historical record.

### The append-only rule (this is the important part)

Once an ADR lands `Active`, its **Decision / Rationale / Alternatives are
invariant**. A future reader must be able to recover what was actually decided
at the time — *even when it later turns out wrong*. Three tiers:

- **Invariant** — never edit the original decision text to reflect later
  understanding. The past stays intact.
- **Allowed maintenance** (additive) — flip `Status:`, add a top forward-pointer,
  append a dated `## Errata` / `## Postscript` at the bottom, fix dead links/typos.
- **Forbidden** — inline "revision notes" that re-narrate the body mid-decision
  (*"Update 2026-Q3: actually D-2 means…"*). If a clarification needs more than a
  postscript, write a **new** ADR and supersede the old one.

Numbering is sequential and **never reused**, even after supersession — readers
rely on stable identifiers.

---

## OpenSpec — the present-state canon and the workflow

OpenSpec serves two distinct roles. Don't conflate them.

### 1. `openspec/specs/<capability>/` — long-lived canon

**Tense:** present. **Owns:** what the system is today.

- **`spec.md`** — the validator-parsed contract. **Synced output — do not
  hand-edit.** It is regenerated when a change is archived; direct edits get
  clobbered and skip validator review.
- **`architecture.md`** — optional, hand-edited. Present-tense schemas,
  invariants, internal interfaces. Opens with a forward-pointer to the binding
  ADR(s) ("the binding decisions are recorded in ADR-NNNN").
- **`demo.md`** — optional worked example for acceptance review.

A capability earns its own directory only if it is a vertical feature with a
persistent contract, or a genuinely cross-cutting contract. Pure
platform/runtime/policy decisions stay ADR-only.

### 2. `openspec/changes/<change-name>/` — the unit of work

**Tense:** future/in-flight. **Owns:** one scoped change.

This is where actual development happens. A change workspace contains:

- **`proposal.md`** — what and why; names the PRD/ADR/spec touchpoints it affects
- **`design.md`** — the technical approach
- **`specs/<capability>/spec.md`** — the **delta** (`ADDED` / `MODIFIED` /
  `REMOVED` requirements)
- **`tasks.md`** — the implementation checklist, including tasks to update canon

Lifecycle:

```bash
openspec new change <change-name>     # scaffold the workspace
# ... propose → design → spec deltas → tasks → implement ...
openspec archive <change-name>        # sync delta specs into openspec/specs/
```

**Every change must name its canonical touchpoints** (which PRD/ADR/spec
surfaces it affects) or explicitly say `None`. On archive, the accepted delta
specs are synced into the canonical `openspec/specs/` — which is exactly why you
never hand-edit `spec.md`.

---

## Decision guide: which format?

| You are… | Use |
|---|---|
| Defining *what* the product is / who it serves | **PRD** |
| Making a cross-cutting or load-bearing technical decision | **ADR** |
| Doing any actual implementation work | **OpenSpec change** |
| Describing what a capability *is* today | **OpenSpec spec** (via a change) |
| Recording capability-local shape (one feature only) | the capability's `architecture.md` |

Common flow for a new feature:

1. **PRD** says the product should do *X* (intent).
2. An **OpenSpec change** is opened to build it (`proposal → design → tasks`).
3. If the change makes a cross-cutting decision, an **ADR** records *why*.
4. On archive, the change's delta syncs into the **OpenSpec spec** (present truth).
5. The ADR carries a forward-pointer to that spec; the spec cites the ADR. Loop closed.

---

## The one-line mental model

- **PRD** = the future we *intend*.
- **ADR** = *why* we decided, frozen at decision time.
- **OpenSpec change** = the work *in flight*.
- **OpenSpec spec** = the present we *have*, synced from finished changes.

_Source: adapted from `b4arena/spellkave/specifications/` (PRD/ADR temporal axis)
and `b4arena/spellkave/openspec/` (OpenSpec workflow)._
