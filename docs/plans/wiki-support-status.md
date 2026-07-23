# Wiki Support Status

**Status**: Implemented through the existing authenticated raw-HTTP layer

Wiki support no longer depends on adding wiki methods to `forgejo-sdk`. The MCP server
uses the same direct REST helpers already employed when the SDK does not expose a Forgejo
endpoint.

The implementation provides six tools:

- `list_wiki_pages`
- `get_wiki_page`
- `get_wiki_revisions`
- `create_wiki_page`
- `update_wiki_page`
- `delete_wiki_page`

It also registers the resource template
`forgejo://repo/{owner}/{repo}/wiki/{pageName}`. Use the server-normalized `page_name`
returned by list/create operations. Percent-encode `/` in slash-separated names as
`%2F` when constructing a resource URI.

See [wiki-support.md](./wiki-support.md) for the superseded SDK contribution plan and
`openspec/changes/add-wiki-support/` for the implementation contract and verification
record.
