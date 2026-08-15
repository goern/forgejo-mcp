<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## MODIFIED Requirements

### Requirement: Upload an attachment to a release

The system SHALL expose an MCP tool `create_release_attachment` that uploads a
new attachment to a release. The tool SHALL accept required `owner`, `repo`,
and `release_id`; optional `content`, `file_path`, `filename`, and `mime_type`;
and SHALL require exactly one of `content` or `file_path`. `filename` SHALL be
required with `content` and SHALL default to the path basename with
`file_path`.

#### Scenario: Successful base64 upload

- **WHEN** the caller supplies valid base64 `content` and a `filename`
- **THEN** the system SHALL decode and upload the content
- **AND** return the new attachment

#### Scenario: Successful path upload

- **WHEN** the caller supplies a `file_path` on the MCP host
- **THEN** the system SHALL stream the file through multipart/form-data
- **AND** return the new attachment

#### Scenario: Invalid base64 content

- **WHEN** the caller supplies invalid base64 `content`
- **THEN** the system SHALL identify the decoding failure without contacting Forgejo
