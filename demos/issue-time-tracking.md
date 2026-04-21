# Demo: issue time tracking tools

*2026-04-21T22:45:00Z by Showboat 0.6.1*
<!-- showboat-id: f2c9b81a-issue-time-tracking-2026 -->

## What these tools do

Six MCP tools give agents a full read/write view of Forgejo's tracked-time ledger on issues and pull requests:

- `list_issue_tracked_times` — List entries on one issue or PR (supports `since`/`before` filters).
- `list_repo_tracked_times` — List entries across a repository (also supports a `user` filter).
- `list_my_tracked_times` — List your own entries across all repositories.
- `add_issue_time` — Log time against an issue or PR. Accepts **either** `seconds` **or** `duration` (e.g. `"15m"`, `"1h30m"`), never both.
- `reset_issue_time` — Delete **all** entries on one issue or PR. Destructive; affects entries from other users too.
- `delete_issue_time_entry` — Delete a single entry by its numeric ID.

Forgejo unifies the issue and PR index namespace, so the same tools work against a PR just by passing the PR number as `index`.

## Setup

Set `FORGEJO_URL` and `FORGEJO_ACCESS_TOKEN` (or use direnv), then:

```bash
make build
```

## CLI mode: parameter schemas

```bash
./forgejo-mcp --cli add_issue_time --help 2>/dev/null
```

```output
Tool: add_issue_time
Description: Log time against an issue or pull request. Provide exactly one of 'seconds' or 'duration'.

Parameters:
  created_at           string     optional   Optional RFC3339 timestamp for when the work happened (defaults to server time)
  duration             string     optional   Time as a duration string, e.g. "15m", "1h30m", "45s". Provide exactly one of seconds or duration.
  index                number     required   Issue or pull request index (Forgejo shares index namespace between the two)
  owner                string     required   Repository owner
  repo                 string     required   Repository name
  seconds              number     optional   Time in seconds to log (positive integer). Provide exactly one of seconds or duration.
  user_name            string     optional   Optional username to log time on behalf of (requires admin; omit for self)
```

## End-to-end workflow

### Step 1: Create a scratch issue to demo against

```bash
./forgejo-mcp --cli create_issue \
  --args '{"owner":"goern","repo":"forgejo-mcp","title":"demo: time tracking","body":"Scratch issue for a Showboat demo."}' 2>/dev/null
```

```output
  Issue #200 created: demo: time tracking
```

### Step 2: Log 15 minutes via `duration`

```bash
./forgejo-mcp --cli add_issue_time \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":200,"duration":"15m"}' 2>/dev/null
```

```output
  Tracked time entry created:
    id: 41
    time: 900 seconds
    user: goern
```

### Step 3: Log 30 more minutes via `seconds`

```bash
./forgejo-mcp --cli add_issue_time \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":200,"seconds":1800}' 2>/dev/null
```

```output
  Tracked time entry created:
    id: 42
    time: 1800 seconds
    user: goern
```

### Step 4: List entries on the issue

```bash
./forgejo-mcp --cli list_issue_tracked_times \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":200}' 2>/dev/null
```

```output
  2 tracked time entries on issue #200:
    id=41  time=900s   user=goern  created=2026-04-21T22:45:12Z
    id=42  time=1800s  user=goern  created=2026-04-21T22:45:15Z
  Total: 2700 seconds (45 minutes)
```

### Step 5: Delete one entry by ID

```bash
./forgejo-mcp --cli delete_issue_time_entry \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":200,"time_id":41}' 2>/dev/null
```

```output
  Delete time entry success
```

```bash
./forgejo-mcp --cli list_issue_tracked_times \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":200}' 2>/dev/null
```

```output
  1 tracked time entry on issue #200:
    id=42  time=1800s  user=goern  created=2026-04-21T22:45:15Z
```

### Step 6: Nuke the whole ledger with `reset_issue_time`

```bash
./forgejo-mcp --cli reset_issue_time \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":200}' 2>/dev/null
```

```output
  Reset tracked time success
```

```bash
./forgejo-mcp --cli list_issue_tracked_times \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":200}' 2>/dev/null
```

```output
  0 tracked time entries on issue #200.
```

### Step 7: Validation failure — both `seconds` and `duration` rejected

```bash
./forgejo-mcp --cli add_issue_time \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":200,"seconds":60,"duration":"1m"}' 2>/dev/null
```

```output
  Error: provide exactly one of 'seconds' or 'duration', not both
```

This check runs client-side before any HTTP call, so the agent sees a structured error instead of a 400 from the API.

### Step 8: Repo-wide and user-scoped queries

```bash
./forgejo-mcp --cli list_repo_tracked_times \
  --args '{"owner":"goern","repo":"forgejo-mcp","user":"goern","since":"2026-04-21T00:00:00Z"}' 2>/dev/null

./forgejo-mcp --cli list_my_tracked_times --args '{}' 2>/dev/null
```

Each returns a JSON array of `TrackedTime` objects across scopes larger than a single issue.

### Step 9: Clean up

```bash
./forgejo-mcp --cli issue_state_change \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":200,"state":"closed"}' 2>/dev/null
```

## Notes

- **Issues vs PRs**: every tool above also accepts a PR index. `add_issue_time --index 101` would log time against PR #101 with no change in parameters.
- **`reset_issue_time` is destructive**: it removes entries from other users too. The tool description flags this explicitly so agents do not reach for it casually.
- **Stopwatch companion**: see [`demos/issue-stopwatch.md`](issue-stopwatch.md) for the live-timer flow that writes tracked-time entries automatically.
