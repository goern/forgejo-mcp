## ADDED Requirements

### Requirement: Repo-scoped listing rejects an empty repository

`list_repo_issues` SHALL validate that `owner` and `repo` are non-empty before calling
upstream. When either is empty, it SHALL return an error naming the offending parameter, and
that error SHALL NOT expose SDK-internal wording such as `path segment [1] is empty`.

When `repo` is empty, the error SHALL direct the caller to `search_issues` for owner-wide
listing.

#### Scenario: Empty repo argument

- **WHEN** `list_repo_issues` is called with a non-empty `owner` and an empty `repo`
- **THEN** the error names `repo` as required
- **AND** the error mentions `search_issues` as the tool for owner-wide listing
- **AND** the phrase `path segment` does not appear in the error

#### Scenario: Empty owner argument

- **WHEN** `list_repo_issues` is called with an empty `owner`
- **THEN** the error names `owner` as required
- **AND** no upstream request is made

#### Scenario: Well-formed call unaffected

- **WHEN** `list_repo_issues` is called with non-empty `owner` and `repo`
- **THEN** it behaves exactly as before this change
