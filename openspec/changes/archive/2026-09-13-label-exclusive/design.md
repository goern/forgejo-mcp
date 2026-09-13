# SPDX-License-Identifier: GPL-3.0-or-later

## Context

See proposal.md Why. `label-crud` (still unarchived) omitted `exclusive` / `is_archived` because SDK v3.0.0 `Label` / `CreateLabelOption` / `EditLabelOption` do not model them; org raw-HTTP omitted them for parity. Its design reserved the revisit: raw-HTTP passthrough on **both** paths, optional params, **no third code path**. This change is that revisit, not a new tool family.

Repo CRUD today is SDK in `operation/issue/label.go`. Org is already `DoJSON`. List and label resources still decode into SDK `Label`, so Exclusive labels come back without the flag even when Forgejo sent it.

## Goals / Non-Goals

**Goals:**

- One local DTO and one JSON contract for repo and org.
- PATCH omitempty on writes; always-present booleans on reads.
- Create-time scoped-name rule for `exclusive=true`; no extra GET on exclusive-only edit.
- Delete stays on the SDK.

**Non-Goals:**

- Archiving `label-crud` or rewriting its “via SDK CreateLabel” sentence.
- SDK upgrade.
- New URIs, assignment tools, label templates.
- Changing list `page`/`limit` envelopes.

## Decisions

### D1: Recorded revisit, not a new family

Capability `label-exclusive`. Transport is how. Tools keep their names. `label-crud` is not Modified and is not archived here.

### D2: Both flags because they are one form

`exclusive` and `is_archived` are the two checkboxes on Edit label, one JSON object, one `*bool` PATCH. Splitting `is_archived` copies the test matrix. Dropping it from a new DTO would hide a field we parse (`#527` / `website`).

### D3: Raw HTTP on repo too

SDK v3.0.0 cannot marshal the fields. Keeping SDK when the flags are omitted is the third path `label-crud` forbade. Org already uses `DoJSON`. Repo create/edit/get/list and label resources switch to `DoJSON` + the same DTO. `forgejo.APIPath` on every path.

Delete stays SDK: exclusive is not on that wire. Delete’s in-use GET may keep using SDK `GetRepoLabel` for the name.

### D4: PATCH omitempty vs read always-present

Create/edit options use `*bool` with `json:"exclusive,omitempty"` / `json:"is_archived,omitempty"` and `ok` + `ptr.To`, same as `edit_repo`. Read DTO and resource payload use `bool` **without** omitempty so `"exclusive":false` is visible.

### D5: Scoped name at the MCP boundary

Forgejo: a label is scoped when the name contains `/` not at either end. `exclusive=true` without a scope does nothing useful in the UI. Reject at create (and on edit when a new name is supplied) with no HTTP, like invalid color. Message states that rule.

Edit without a new name does **not** GET to re-validate. Document the `/` rule on the tool.

### D6: Output bounding exempt

Create/edit/get return one object. List tools already have `page`/`limit`. This change does not add a list.

### D7: Live host is the instance that was probed

Demo and live table name a Forgejo 16.x instance actually used (`git.b4mad.industries` or another), never a leftover Codeberg URL.

## Risks / Trade-offs

- [List JSON grows two keys] → Mitigation: booleans, always present; agents can ignore them.
- [Exclusive-only edit of an unscoped label] → Forgejo may ignore Exclusive; we do not extra-GET. Documented.
- [SDK GetRepoLabel in delete vs DoJSON get tool] → Two transports on delete’s preflight only. Accepted: delete is out of this wire.

## Migration Plan

Additive. Existing create/edit calls omit the new keys. After merge, Exclusive labels created through MCP round-trip on list/get.

## Open Questions

None. Live demo host is filled when the demo runs, not a spec fork.
