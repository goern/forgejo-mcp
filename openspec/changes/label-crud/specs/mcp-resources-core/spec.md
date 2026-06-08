## ADDED Requirements

### Requirement: Collection resource

A resource template MAY expose a **bounded collection** of entities as a top-level resource (e.g. `forgejo://repo/{owner}/{repo}/labels`), in addition to single-entity resources. This is the framework-level answer to "may a list be a resource, not only a tool": yes, provided it is bounded exactly like an embedded list. A collection resource SHALL cap its items at `EmbeddedListCap` (default 30) and SHALL append the same truncation sentinel as embedded lists, naming the existing list tool callers use to enumerate the remainder. A collection resource SHALL NOT remove or replace the corresponding list tool, which remains the unbounded enumeration path.

Collection-resource URIs SHALL use the plural collection segment with no per-entity key (e.g. `…/labels`), distinguishing them from the singular single-entity form (`…/label/{id}`).

#### Scenario: Collection resource under cap returns in full
- **WHEN** a client reads a collection resource for a repo with `M` entities where `M ≤ 30`
- **THEN** the response SHALL include all `M` entities
- **AND** no truncation sentinel SHALL appear

#### Scenario: Collection resource over cap is truncated with sentinel
- **WHEN** a client reads a collection resource for a repo with `M` entities where `M > 30`
- **THEN** the response SHALL include at most 30 entities
- **AND** the response SHALL append a truncation sentinel naming the corresponding list tool

#### Scenario: Collection resource does not remove the list tool
- **WHEN** a collection resource is registered for an entity that already has a list tool
- **THEN** that list tool SHALL still appear in `tools/list`
- **AND** SHALL continue to respond identically
