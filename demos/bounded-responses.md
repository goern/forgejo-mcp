# Demo: bounded responses for get_pull_request_diff + get_file_content

*2026-05-12T10:50:00Z by Showboat 0.6.1*
<!-- showboat-id: 9e4d3c81-bounded-responses-demo-2026 -->

## Background

Two existing tools returned arbitrarily large payloads:

- `get_pull_request_diff` — the full unified diff for the PR, every file
  concatenated. A 30-file PR easily blows past 30 kB.
- `get_file_content` (plain-text mode) — the entire file, no matter how
  large.

The architectural rule for this MCP server
([docs/design/output-bounding.md](../docs/design/output-bounding.md))
says every data-proportional response must be bounded by the caller.
Issue [#124](https://codeberg.org/goern/forgejo-mcp/issues/124) made it
the law for these two.

v2.22.0 adds optional, additive parameters:

- `get_pull_request_diff` → `file_path` selects a single file's hunks.
- `get_file_content` → `start_line` + `end_line` select a 1-indexed
  inclusive line range. Out-of-range values clamp.

Default behavior is unchanged when the new params are omitted.

## Setup

```bash
export FORGEJO_URL=https://codeberg.org
export FORGEJO_ACCESS_TOKEN=<your-token>
make build
```

## 1. get_pull_request_diff — full vs. per-file

Full diff of PR [#131](https://codeberg.org/goern/forgejo-mcp/pulls/131)
(the PR that added this very feature):

```bash
./forgejo-mcp --cli get_pull_request_diff \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":131}' 2>/dev/null | python3 -c "
import sys, json
text = json.load(sys.stdin)[0]['text']
print(f'Full diff: {len(text):,} bytes, {text.count(chr(10)):,} lines')
"
```

```output
Full diff: 44,720 bytes, 988 lines
```

Now slice just one file:

```bash
./forgejo-mcp --cli get_pull_request_diff \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":131,"file_path":"pkg/diff/splitter.go"}' 2>/dev/null | python3 -c "
import sys, json
text = json.load(sys.stdin)[0]['text']
print(f'Sliced: {len(text):,} bytes, {text.count(chr(10)):,} lines')
print()
print(chr(10).join(text.split(chr(10))[:8]))
"
```

```output
Sliced: 2,731 bytes, 86 lines

diff --git a/pkg/diff/splitter.go b/pkg/diff/splitter.go
new file mode 100644
index 0000000..f05ad55
--- /dev/null
+++ b/pkg/diff/splitter.go
@@ -0,0 +1,81 @@
+// Package diff offers small helpers for slicing unified-diff strings.
+// It exists so MCP tools that return diff payloads can satisfy the
```

**16× smaller payload** for the file the agent actually wanted to review.

### Missing path → tool error

```bash
./forgejo-mcp --cli get_pull_request_diff \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":131,"file_path":"nonexistent/file.go"}' 2>&1 | tail -1
```

```output
Error: tool execution failed: file_path "nonexistent/file.go" not found in pull request diff
```

The caller should use `list_pull_request_files` first to discover the
exact paths in the PR.

## 2. get_file_content — full vs. line range

Full CHANGELOG.md:

```bash
./forgejo-mcp --cli get_file_content \
  --args '{"owner":"goern","repo":"forgejo-mcp","ref":"main","filePath":"CHANGELOG.md"}' 2>/dev/null | python3 -c "
import sys, json
r = json.load(sys.stdin)
text = json.loads(r[0]['text']).get('Result', '')
print(f'Full CHANGELOG: {len(text):,} chars, {text.count(chr(10)):,} lines')
"
```

```output
Full CHANGELOG: 38,774 chars, 545 lines
```

Slice lines 1–10:

```bash
./forgejo-mcp --cli get_file_content \
  --args '{"owner":"goern","repo":"forgejo-mcp","ref":"main","filePath":"CHANGELOG.md","start_line":1,"end_line":10}' 2>/dev/null | python3 -c "
import sys, json
r = json.load(sys.stdin)
text = json.loads(r[0]['text']).get('Result', '')
print(f'Sliced: {len(text):,} chars, {text.count(chr(10))+1:,} lines')
print()
print(text)
"
```

```output
Sliced: 936 chars, 10 lines

## [2.22.0](https://codeberg.org/goern/forgejo-mcp/compare/v2.21.0...v2.22.0) (2026-05-12)

### :sparkles: Features

* add list_org_labels + merge org labels in list_repo_labels (#130)
* bounded responses for get_pull_request_diff + get_file_content (#131)
...
```

**41× smaller payload** for the section the agent wanted to read.

### Range shortcuts

| Args                       | Behavior                              |
|----------------------------|---------------------------------------|
| (neither set)              | Full file (unchanged contract)        |
| `start_line=N`             | From line N to EOF                    |
| `end_line=N`               | From line 1 to N                      |
| `start_line=A, end_line=B` | Inclusive `[A..B]`, clamped to extent |
| `start_line > end_line`    | Tool error                            |

Slicing applies only to plain-text mode. `with_metadata=true` returns
the SDK `ContentsResponse` unchanged (base64-encoded content + sha) —
the slice would break the encoding, so the range params are silently
ignored when metadata mode is set.

## 3. End-to-end: agent code-review workflow

Reviewing a PR no longer means pulling its entire diff and burning
context window on files the agent does not care about:

1. `list_pull_request_files` — discover `filename`s.
2. For each file, decide whether to read it.
3. `get_pull_request_diff` with `file_path=<filename>` — just that
   file's hunks.
4. If the agent needs the surrounding source, `get_file_content` with
   `start_line` + `end_line` around the hunk — not the whole file.

Per-call payloads stay proportional to what the agent actually
inspects, not to the size of the PR or the file. This is the rule
codified in [docs/design/output-bounding.md](../docs/design/output-bounding.md)
and now enforced for both tools.
