<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## MODIFIED Requirements

### Requirement: Resumable response envelope

`search_issues` SHALL return a JSON object, not a bare array, carrying the issues together
with a continuation signal: the `page` and `limit` actually used, the `count` of issues in
this response, a boolean `has_next`, and an optional `total_count`.

`limit` in the envelope SHALL report the value actually sent to the upstream search request
(the effective limit), sourced from the same value used to build that request, not
recomputed independently from the caller's raw input.

`has_next` derivation depends on whether the instance's pagination ceiling is known:

- When the ceiling is known, the Caller-controlled bound requirement guarantees the effective
  `limit` never exceeds it, so the request is always honored in full and `has_next` SHALL be
  `count == limit`.
- When the ceiling is unknown, `has_next` SHALL be derived by a pagination-preserving
  next-page probe: when `count` is 0, `has_next` SHALL be false; otherwise the handler SHALL
  request `page+1` at the **same** `limit` and set `has_next` according to whether the probe
  returned any issues. The probe MUST NOT request `limit+1`: Forgejo derives page offsets from
  the page size, and changing it causes later pages to skip rows. This is the rule
  `openspec/specs/wiki-tools/spec.md:31-38` establishes for `list_wiki_pages`, widened here
  from "probe on a full page" to "probe on any non-empty page", because `search_issues`
  advertises a `limit` callers can set above the (here, unconfirmed) ceiling.

The probe SHALL NOT run when the ceiling is known and the request was accepted: a short page
is unambiguous in that case (the limit was honored in full, so nothing was clamped), and
running it would cost an unnecessary upstream request.

`total_count` SHALL be populated from the upstream response's `X-Total-Count` header via
`pkg/forgejo.TotalCountPtr`, carrying the total number of matching issues across every page
(distinct from `count`, which is only the row count in this response). The key SHALL be
omitted (`omitempty`) — never emitted as `0` — when the header is absent or does not parse
as a non-negative integer, so "unknown" is never confused with "confirmed zero results."

#### Scenario: More results available (ceiling unknown)

- **WHEN** the instance's ceiling is unknown, a call returns issues, and the next-page probe
      at the same `limit` returns at least one issue
- **THEN** `has_next` is true
- **AND** the caller can retrieve the remainder by re-issuing the call with `page` incremented

#### Scenario: Last page reached (ceiling unknown)

- **WHEN** the instance's ceiling is unknown and a call returns no issues, or the next-page
      probe at the same `limit` returns none
- **THEN** `has_next` is false

#### Scenario: Full page under a known ceiling still signals continuation

- **WHEN** the instance's ceiling is known, a call with an accepted `limit` returns exactly
      `limit` issues, and further matching issues exist
- **THEN** `has_next` is true
- **AND** no next-page probe request is made

#### Scenario: Clamped page still signals continuation (unknown-ceiling fallback only)

- **WHEN** the instance's ceiling is unknown, a call with `limit` 100 is answered with 50
      issues because the instance silently clamps at its own `MAX_RESPONSE_ITEMS`, and
      further matching issues exist
- **THEN** `has_next` is true, as determined by the next-page probe
- **AND** this scenario cannot occur when the ceiling is known, because a `limit` that would
      be clamped is rejected before the upstream call (see Caller-controlled bound)

#### Scenario: Envelope always present

- **WHEN** any successful `search_issues` call completes
- **THEN** the response object contains `issues`, `page`, `limit`, `count`, and `has_next`
- **AND** `count` equals the length of `issues`
- **AND** `limit` equals the value actually sent upstream

#### Scenario: Total count surfaced when Forgejo reports it

- **WHEN** the upstream response carries `X-Total-Count: 137`
- **THEN** the envelope SHALL contain `total_count: 137`

#### Scenario: Total count omitted, not zeroed, when Forgejo does not report it

- **WHEN** the upstream response carries no `X-Total-Count` header, or an unparsable one
- **THEN** the envelope SHALL NOT contain a `total_count` key at all
- **AND** SHALL NOT report `total_count: 0`
