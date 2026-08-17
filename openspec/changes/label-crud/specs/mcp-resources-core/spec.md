## ADDED Requirements

### Requirement: Collection resource

A resource template MAY expose a **bounded collection** of entities as a top-level resource (e.g. `forgejo://repo/{owner}/{repo}/labels{?page,limit}`), in addition to single-entity resources. This is the framework-level answer to "may a list be a resource, not only a tool": yes, provided it is bounded exactly like an embedded list. A collection resource SHALL accept optional `page` and `limit` query parameters as the **client-controlled bound**, and SHALL cap its items at `EmbeddedListCap` (default 30) as a hard ceiling rather than as the only bound. It SHALL append the same truncation sentinel as embedded lists, naming the existing list tool callers use to enumerate the remainder. A collection resource SHALL NOT remove or replace the corresponding list tool, which remains the unbounded enumeration path.

A collection resource SHALL register its template **with** the `{?page,limit}` expansion. This is load-bearing, not documentation: mcp-go matches a read against an anchored regexp built from the registered template string, so a template registered bare matches only the bare URI and every query-bearing read fails before any handler runs.

A collection resource SHALL request exactly the caller's `limit` from upstream. It SHALL NOT over-fetch (`limit+1`) as a truncation probe: upstream computes the offset as `(page-1)*PageSize`, so requesting one extra row while showing only `limit` of them makes page N+1 begin one row past the last row page N showed — a row no page can return. Positively stated, and independent of mechanism: successive pages SHALL partition the collection, every entity reachable by exactly one page.

"More exists" SHALL therefore be determined from what the response already carries, not by over-fetching. Against Forgejo that is `Link … rel="next"`, with `X-Total-Count` for the total; an endpoint that reports it another way (an SDK next-page field, a count, a cursor) satisfies this requirement equally.

Collection-resource URIs SHALL use the plural collection segment with no per-entity key (e.g. `…/labels`), distinguishing them from the singular single-entity form (`…/label/{id}`).

#### Scenario: Collection resource under cap returns in full
- **WHEN** a client reads a collection resource **without `page` or `limit`** for a repo with `M` entities where `M ≤ 30`
- **THEN** the response SHALL include all `M` entities
- **AND** no truncation sentinel SHALL appear

#### Scenario: Query-bearing read routes and honours the caller's limit
- **WHEN** a client reads a collection resource with `?limit=N` for a repo with more than `N` entities
- **THEN** the read SHALL be routed to the resource's handler rather than failing as an unknown resource
- **AND** the response SHALL include exactly `N` entities
- **AND** the response SHALL append the truncation sentinel

#### Scenario: Successive pages partition the collection
- **WHEN** a client reads pages `1..P` of a collection resource at `?limit=N`
- **THEN** the entities returned SHALL be the first `P × N` of the collection, in order
- **AND** no entity SHALL be skipped at a page boundary
- **AND** no entity SHALL appear on more than one page

#### Scenario: Collection resource over cap is truncated with sentinel
- **WHEN** a client reads a collection resource for a repo with `M` entities where `M > 30`
- **THEN** the response SHALL include at most 30 entities
- **AND** the response SHALL append a truncation sentinel naming the corresponding list tool

#### Scenario: Collection resource does not remove the list tool
- **WHEN** a collection resource is registered for an entity that already has a list tool
- **THEN** that list tool SHALL still appear in `tools/list`
- **AND** SHALL continue to respond identically
