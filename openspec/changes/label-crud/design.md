## Context

`forgejo-mcp` has label **read** (`list_repo_labels`, `list_org_labels`) and **assignment** (`add_issue_labels`, `remove_issue_labels`) tools in `operation/issue/issue.go`, but no label lifecycle management. Repo-label CRUD endpoints (`GET`/`POST`/`PATCH`/`DELETE /repos/{owner}/{repo}/labels[/{id}]`) are Gitea-compatible and already wrapped by the pinned `codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3`. Org-label endpoints (`/orgs/{org}/labels[/{id}]`) share the shape but have **no** SDK method.

The `mcp-resources-core` capability (defined by the not-yet-archived `mcp-resource-templates` change) established the `forgejo://` scheme, the `operation/resource` helper package (`ParseXxx`, `Bounded`, `EmbeddedListCap`, `MapForgejoError`), and the output-bounding contract. This change consumes that machinery rather than extending it.

## Goals / Non-Goals

**Goals**

- Repo- **and org**-label create / edit / delete / get-one as MCP tools — repo over existing SDK methods, org over `pkg/forgejo` raw-HTTP `DoJSON`.
- Safe delete: `delete_mode` enum + in-use guard on both delete tools (D9).
- Three additive, URI-addressable label resource-templates (repo single, repo list, org list) consistent with the issue/pr/commit precedent.
- Zero new dependencies; reuse the singleton client and the `operation/resource` helpers verbatim.

**Non-Goals**

- `exclusive` / `is_archived` fields (not modelled by SDK v3 `Label`; org raw-HTTP omits them for parity).
- Any change to the existing list/assignment tools.

## Decisions

### D1: Repo-label CRUD via SDK, org-label CRUD via raw-HTTP `DoJSON`

The pinned SDK exposes `CreateLabel(owner, repo, CreateLabelOption)`, `EditLabel(owner, repo, id, EditLabelOption)`, `DeleteLabel(owner, repo, id)`, `GetRepoLabel(owner, repo, id)`. Wrapping these is a thin, well-understood pattern matching every other SDK-backed tool. Org labels have **no** SDK method, so the org tools go through `pkg/forgejo` raw-HTTP `DoJSON` against `/orgs/{org}/labels[/{id}]`, mirroring the existing `fetchOrgLabels`. Both implementation styles already exist in the codebase (SDK wrappers and `DoJSON` callers), so carrying both in one change is consistent with the wiki/attachment precedent rather than a new pattern. Org and repo tools share the color-normalisation, PATCH, and safe-delete helpers (D9) so the two paths differ only in transport.

### D2: `get_repo_label` added alongside CRUD, not just create/edit/delete

`list_repo_labels` lists but there is no read-one. The resource template `forgejo://repo/{owner}/{repo}/label/{id}` needs a single-label fetch internally; exposing the same as a tool (`get_repo_label`) fills the read-one gap for tool-only clients at no extra cost (same SDK call, `GetRepoLabel`).

### D3: Color normalisation before send

Forgejo's `CreateLabelOption.Color` / `EditLabelOption.Color` expect a hex string. The API `422`s on a bare `rrggbb` without the leading `#`. The tools normalise: accept `rrggbb` or `#rrggbb`, lowercase, prepend `#` if absent, reject anything that is not a valid 6-digit hex color with `-32602` (invalid params) rather than forwarding a guaranteed `422`. This keeps the failure at the MCP boundary with a clear message. **3-digit shorthand (`#abc`) is deliberately NOT accepted**: Gitea's label-color validation is not confirmed to accept it, and silently expanding `#abc`→`#aabbcc` would be undefined reinterpretation. Six-digit only until upstream behaviour is verified (see Open Questions).

### D4: `edit_repo_label` is PATCH — only supplied fields change

`name`, `color`, `description` are all optional on `edit_repo_label`. Unsupplied fields are omitted from `EditLabelOption` so the upstream PATCH leaves them untouched. Supplying none is a no-op error (`-32602`) rather than a wasted round-trip.

### D5: Label domain placement — implementer's call, defaults to issue domain

Labels currently live in `operation/issue/issue.go`. The new tools and `resources*.go` MAY stay in the issue domain (cohesion with existing label code) or move to a new `operation/label/` domain (separation of a growing surface). The spec is deliberately **layout-agnostic**: it asserts behavior and registration, never file location. The implementer chooses; a later move is a refactor, not a spec change. No default is prescribed.

### D6: Label resources follow `mcp-resources-core` verbatim

- `forgejo://repo/{owner}/{repo}/label/{id}` → single `application/json` block; `id` parsed by a new `ParseLabel` helper rejecting non-numeric ids with `-32602`.
- `forgejo://repo/{owner}/{repo}/labels` → bounded embedded list. Request `EmbeddedListCap+1` items so `Bounded`'s `>cap` check fires; the truncation sentinel names the `list_repo_labels` tool as the enumeration fallback (mirrors the issue-domain `resources.go` pattern).
- Both reuse the singleton `pkg/forgejo` client; private-repo reads map `403`→`-32002`, `404`→`-32003` via `MapForgejoError`.

### D7: `labels` (plural) list vs `label/{id}` (singular) — follow the established scheme

The MCP specification itself imposes **no** URI naming convention: resource URIs are opaque strings and templates are RFC 6570 patterns, so "singular vs plural" is not an MCP-level question. The convention to follow is therefore the project's own `forgejo://` scheme, defined in `mcp-resources-core`: single-entity reads use a **singular** segment plus a key (`commit/{sha}`, `issue/{index}`, `pr/{index}`). The single-label URI conforms: `label/{id}`.

The list URI `…/labels` is **plural** because it names a collection and carries no per-entity key — there is no singular form to apply. This is not a divergence from the scheme; it is the collection form the scheme did not previously need (the seven existing templates are all single-entity). See D8 for the larger point this raises.

### D8: First standalone collection resource — precedent worth recording

`forgejo://repo/{owner}/{repo}/labels` is the **first collection-as-resource** in the scheme. Every existing resource is a single-entity fetch; variable-size lists have so far lived *embedded inside* an entity (an issue's `recent_comments`, a commit status's `statuses`), never as a top-level resource. The proposal rationale for `mcp-resources-core` even leans "bounded lists stay tools, resources fetch single entities" — and `list_repo_labels` already exists as that tool.

Issue #190 explicitly asks for the labels **resource** anyway (URI-addressable discoverability, parity with the embedded-list bounding contract). That is a reasonable ask, but it sets a precedent: it answers "may a bounded collection be a resource, not only a tool?" with yes, bounded identically to embedded lists (`EmbeddedListCap` + sentinel naming the list tool). Future list-shaped entities (org labels, branches, milestones, releases) will cite this decision.

Because the precedent is cross-cutting — it belongs to the resource framework, not to labels — the normative home for it is `mcp-resources-core`. **Resolved: promoted.** This change adds a `Collection resource` requirement to the `mcp-resources-core` capability (see `specs/mcp-resources-core/spec.md` and the Modified Capabilities + Dependency sections of `proposal.md`). The label list is its first instance; future list entities cite the core requirement, not this design note.

### D9: Safe delete — `delete_mode` enum + in-use guard

Deleting a label is silently lossy: the Forgejo API unconditionally strips the label off every issue/PR carrying it, with no undo via MCP. For labels that encode triage/priority state that is real data loss. So `delete_repo_label` / `delete_org_label` **refuse by default** when the label is in use and report the reference count; `delete_mode="force"` overrides. This trades one extra read per delete for a guardrail on an irreversible action — acceptable, since delete is rare.

Counting the references:

- **Repo label** — count via the SDK `ListRepoIssues` call filtered by the label name (`state=all`, `type=all`), reading the page total from the returned `*forgejo_sdk.Response` (which parses Gitea's pagination headers). **Note:** `pkg/forgejo.DoJSON` decodes only the response body and does not expose `resp.Header`, so the raw `X-Total-Count` header is **not** reachable through the existing raw-HTTP helper — the count goes through the SDK `*Response`, not a hand-read header. (Filter by label name, not id, per the issues-search API; Gitea counts PRs as issues, so the total covers both.)
- **Org label** — no single endpoint counts org-wide usage, because an org label can be applied in any of the org's repos. The guard iterates the org's repos and sums per-repo counts — O(repos) calls, bounded, only paid on an explicit `delete_org_label`. **This count is best-effort over repos the token can read**: a label applied in a private repo the token cannot list is invisible, so the sum MAY under-count. The reported count and the delete-refusal message SHALL state that the count excludes inaccessible repos. If repo enumeration itself fails, the tool routes the caller to the explicit `delete_mode="force"` override (it does not silently delete) — note this is a routing fallback, not a stronger guarantee: `delete_mode="force"` is still a destructive override, so the org guard is weaker than the repo guard by construction.

The `delete_mode` enum and the count helper are shared by both repo and org delete tools so the two transports differ only in how they fetch and delete, not in the guard semantics.

- **Color validation drift**: if Forgejo later accepts non-hex color names, the strict `-32602` reject would be wrong. Mitigation: validation is one helper; loosening it is a one-line change, and rejecting-early is safer than forwarding a `422` today.
- **Org-label delete cost + accuracy**: the in-use guard iterates the org's repos (D9), O(repos) calls on a large org, and the count is best-effort — it under-counts labels used in repos the token cannot read. Accepted: only paid on an explicit `delete_org_label` (rare admin action), and the refusal message discloses the visibility limit so the user is not given false assurance.
- **Two label transports in one change**: repo via SDK, org via `DoJSON`. Mitigation: both styles already exist in the codebase (wiki/attachment precedent); shared color/PATCH/safe-delete helpers keep the divergence to transport only.
- **Label domain churn**: leaving the domain-split decision open risks a later move. Accepted: the spec is layout-agnostic (D5), so a future move is a refactor, not a spec change.

## Migration Plan

Purely additive. No existing tool or resource changes. Resource-unaware clients keep today's surface; resource-aware clients gain three read paths. Tasks land in two independent slices (tools, then resources); within tools, repo-label CRUD and org-label CRUD can ship in either order.

## Open Questions

- ~~Collection-resource precedent (D8): record in `mcp-resources-core` or defer?~~ **Resolved: promoted to the core spec** (`specs/mcp-resources-core/spec.md`, `Collection resource` requirement). Carries the ordering caveat in `proposal.md` § Dependency: archive after `mcp-resource-templates`.
- ~~`delete_repo_label`: confirm unused before deleting, or delete unconditionally?~~ **Resolved (D9): refuse an in-use label and report the count; `delete_mode="force"` overrides.** Repo count via SDK `ListRepoIssues` pagination (`DoJSON` can't read `X-Total-Count`); org count is best-effort over visible repos and MAY under-count.
- ~~Org-label CRUD: separate change or a second slice here?~~ **Resolved: included in this change** via raw-HTTP `DoJSON` (D1), plus the `forgejo://org/{org}/labels` resource (gate lifted).
- `exclusive` / `is_archived`: still omitted on both transports for the first cut (SDK doesn't model them; org raw-HTTP omits for parity). Raw-HTTP passthrough on both paths is the revisit path if a use case appears.
- **Verify before code — org get-one**: confirm `GET /orgs/{org}/labels/{id}` exists in the pinned Forgejo version. If absent, `get_org_label` SHALL be implemented as list-and-filter over `/orgs/{org}/labels`, and its spec scenario updated to match (different cost profile).
- **Verify before code — color shorthand**: confirm whether the pinned Forgejo accepts 3-digit `#abc`. Current spec is 6-digit-only (fail-closed). If a use case for shorthand appears, add it back only with a normative expansion scenario (`#abc`→`#aabbcc`), never silent.
- **Reserved / future parameters**: `exclusive` and `is_archived` are PLANNED, gated on SDK support (repo) / explicit raw-HTTP modelling (org). When added they extend `create_repo_label`/`edit_repo_label` as **optional** params — a non-breaking additive change — so today's signatures are forward-compatible. Documented here so a future contributor does not ship them through a third code path.

## Adversarial Review — 2026-06-02

Debate team: `adversary` (devils-advocate), `defender` (proponent), `lens-api-evolution`. Teammates grepped the live code; three critiques were code-confirmed (`pkg/forgejo.DoJSON` discards response headers; `forgejo://owner/{owner}` resolves user-first; `operation/resource.Bounded` caps post-fetch).

**Applied (defender conceded, edits made):**

- **C1 — repo in-use count cited an unreachable header.** D9 said "read `X-Total-Count`", but `DoJSON` never returns `resp.Header`. Rerouted to SDK `ListRepoIssues` pagination (`*Response`). (D9, proposal, tasks 1.2, resolved open question.)
- **C2 — org list URI sat on the user-first `owner` root.** Labels are org-only; `forgejo://owner/{user}/labels` would 404 ambiguously. Renamed to `forgejo://org/{org}/labels` (new `org` root, matches `/orgs/{org}/labels`). (proposal, design, spec, tasks.)
- **C3 — org in-use count is unsound under token visibility.** A label used in a private repo the token can't list is invisible → undercount → false assurance. Reframed as best-effort; refusal message now discloses the visibility limit; corrected the "fail-closed" mislabel (`force` is a destructive override, not a block). (D9, risks, spec scenario.)
- **C4 — `EmbeddedListCap=30` with no caller knob contradicted the spec's own output-bounding claim.** Added client-controlled `page`/`limit` to both list resources; `EmbeddedListCap` is now the ceiling, not the only bound. (mcp-resource-label spec, tasks 4.4–4.5.)
- **C5 (partial) — 3-digit hex was an unverified `SHALL`.** Dropped to 6-digit-only (fail-closed); shorthand returns only with a normative expansion scenario. Org get-one endpoint existence + color shorthand are now pre-code verification gates in Open Questions. (D3, spec, tasks 1.1, Open Questions.)
- **Lens — reserved `exclusive`/`is_archived`; scope-less `forgejo://label/{id}` forbidden.** Recorded forward-compat note + parser rejection task. (Open Questions, mcp-resource-label spec, task 4.6.)

**Author-decided (resolved 2026-06-02 by goern):**

- **C6 — cross-modifying the unarchived `mcp-resources-core` (the D8 promotion).** Adversary+defender flagged a dangling-delta risk if `label-crud` archives before `mcp-resource-templates`. **Decision: keep as-is — promotion stands.** `openspec validate --strict` already passes with the delta present, so the tooling does not treat it as invalid; the risk is archive-ordering only and is documented in `proposal.md` § Dependency. Accepted as an unenforced manual ordering promise.
- **`force: bool` → `delete_mode` enum.** Forward-compat hedge for future dry-run/reassign modes. **Decision: adopted the enum.** `delete_repo_label` / `delete_org_label` now take `delete_mode: "safe" | "force"` (default `"safe"`), reserving room for `"dry_run"`/`"reassign"` without a breaking change. Applied across proposal, D9, specs, tasks.
- **Repo tool naming → symmetry.** **Decision: renamed for symmetry.** Repo tools are now `create_repo_label` / `edit_repo_label` / `delete_repo_label` / `get_repo_label`, paralleling the `*_org_label` set. This deviates from issue #190's stated `create_label`/`edit_label`/`delete_label` names — when ticking #190's acceptance criteria, map `create_label`↔`create_repo_label` etc.

**Stalemates:** none.
