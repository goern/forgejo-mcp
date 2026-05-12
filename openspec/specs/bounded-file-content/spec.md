# bounded-file-content Specification

## Purpose
TBD - created by archiving change add-bounded-text-responses. Update Purpose after archive.
## Requirements
### Requirement: Line-range slicing of file content

The `get_file_content` MCP tool SHALL accept optional `start_line` and `end_line` number parameters. Line numbers SHALL be 1-indexed and inclusive on both ends. When both are omitted (or both zero), the tool SHALL return the full file content (current behavior).

When at least one of `start_line` or `end_line` is set, the tool SHALL split the file on `\n`, return the requested slice, and rejoin with `\n`. If `start_line` is omitted but `end_line` is set, `start_line` SHALL default to 1. If `end_line` is omitted but `start_line` is set, `end_line` SHALL default to the file's line count. Out-of-range values SHALL clamp to the file extent rather than erroring (`start_line < 1` → 1; `end_line > line_count` → line_count).

If after clamping `start_line > end_line`, the tool SHALL return an MCP error explaining the inversion.

Line slicing SHALL apply only when `with_metadata` is false (or unset). When `with_metadata=true`, the tool SHALL return the full `ContentsResponse` unchanged regardless of `start_line`/`end_line`, because the response carries base64-encoded content whose semantics cannot be sliced safely server-side.

#### Scenario: Both bounds omitted returns full file

- **WHEN** the tool is called with `owner`, `repo`, `ref`, `filePath` and neither `start_line` nor `end_line`
- **THEN** the system SHALL return the full file content as plain text

#### Scenario: In-range slice

- **WHEN** the tool is called with `start_line=5` and `end_line=10` against a file with 100 lines
- **THEN** the system SHALL return lines 5–10 inclusive, joined with `\n`

#### Scenario: start_line omitted defaults to 1

- **WHEN** the tool is called with `end_line=10` and no `start_line`
- **THEN** the system SHALL return lines 1–10

#### Scenario: end_line omitted defaults to line count

- **WHEN** the tool is called with `start_line=50` and no `end_line` against a file with 100 lines
- **THEN** the system SHALL return lines 50–100

#### Scenario: end_line beyond file extent clamps

- **WHEN** the tool is called with `start_line=90` and `end_line=999` against a file with 100 lines
- **THEN** the system SHALL return lines 90–100 without error

#### Scenario: start_line below 1 clamps

- **WHEN** the tool is called with `start_line=-5` and `end_line=3`
- **THEN** the system SHALL clamp `start_line` to 1 and return lines 1–3

#### Scenario: Inverted range after clamping

- **WHEN** the tool is called with `start_line=20` and `end_line=10`
- **THEN** the system SHALL return an MCP error explaining the inversion

#### Scenario: with_metadata=true ignores slicing

- **WHEN** the tool is called with `with_metadata=true` and `start_line=5`, `end_line=10`
- **THEN** the system SHALL return the full `ContentsResponse` (base64 content + sha + size + links) unchanged

