# Demo: issue stopwatch tools

*2026-04-21T22:45:30Z by Showboat 0.6.1*
<!-- showboat-id: 7a3e9d44-issue-stopwatch-2026 -->

## What these tools do

Four MCP tools let agents drive Forgejo's per-issue live stopwatch — useful when the agent wants to time in-progress work without computing the elapsed seconds itself:

- `start_issue_stopwatch` — Begin timing against an issue or PR. Fails if one is already running on the same issue (Forgejo allows at most one per issue).
- `stop_issue_stopwatch` — Stop the timer and **write the elapsed time as a tracked-time entry**. This is the primary path.
- `cancel_issue_stopwatch` — Cancel the timer and discard the elapsed time. No tracked-time entry is created.
- `list_my_stopwatches` — List every stopwatch currently running for the authenticated user, across all repositories.

Forgejo unifies issue and PR index namespace, so these work on both without parameter changes.

## Setup

```bash
make build
```

## CLI mode: parameter schemas

```bash
./forgejo-mcp --cli start_issue_stopwatch --help 2>/dev/null
```

```output
Tool: start_issue_stopwatch
Description: Start a stopwatch on an issue or pull request. Only one stopwatch per issue; fails if one is already running.

Parameters:
  index                number     required   Issue or pull request index (Forgejo shares index namespace between the two)
  owner                string     required   Repository owner
  repo                 string     required   Repository name
```

```bash
./forgejo-mcp --cli list_my_stopwatches --help 2>/dev/null
```

```output
Tool: list_my_stopwatches
Description: List all currently running stopwatches for the authenticated user
```

## End-to-end workflow

### Step 1: Create a scratch issue

```bash
./forgejo-mcp --cli create_issue \
  --args '{"owner":"goern","repo":"forgejo-mcp","title":"demo: stopwatch","body":"Scratch issue for a Showboat demo."}' 2>/dev/null
```

```output
  Issue #201 created: demo: stopwatch
```

### Step 2: Start a stopwatch

```bash
./forgejo-mcp --cli start_issue_stopwatch \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":201}' 2>/dev/null
```

```output
  Stopwatch started
```

### Step 3: Confirm it is running

```bash
./forgejo-mcp --cli list_my_stopwatches --args '{}' 2>/dev/null
```

```output
  1 running stopwatch:
    repo=goern/forgejo-mcp  issue=#201  "demo: stopwatch"  elapsed≈3s
```

### Step 4: Do some (simulated) work

```bash
sleep 3
```

### Step 5: Stop the stopwatch — elapsed time is recorded as a tracked-time entry

```bash
./forgejo-mcp --cli stop_issue_stopwatch \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":201}' 2>/dev/null
```

```output
  Stopwatch stopped; elapsed time recorded as a tracked time entry
```

```bash
./forgejo-mcp --cli list_issue_tracked_times \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":201}' 2>/dev/null
```

```output
  1 tracked time entry on issue #201:
    id=55  time=3s  user=goern  created=2026-04-21T22:45:38Z
```

### Step 6: Second run — `cancel` discards instead of recording

```bash
./forgejo-mcp --cli start_issue_stopwatch \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":201}' 2>/dev/null
```

```output
  Stopwatch started
```

```bash
sleep 2

./forgejo-mcp --cli cancel_issue_stopwatch \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":201}' 2>/dev/null
```

```output
  Stopwatch cancelled
```

```bash
./forgejo-mcp --cli list_issue_tracked_times \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":201}' 2>/dev/null
```

```output
  1 tracked time entry on issue #201:
    id=55  time=3s  user=goern  created=2026-04-21T22:45:38Z
```

No new entry was created — the 2-second cancelled run was discarded as intended.

### Step 7: Running-stopwatch list is empty again

```bash
./forgejo-mcp --cli list_my_stopwatches --args '{}' 2>/dev/null
```

```output
  No running stopwatches.
```

### Step 8: Try to start two on the same issue — expected error

```bash
./forgejo-mcp --cli start_issue_stopwatch \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":201}' 2>/dev/null

./forgejo-mcp --cli start_issue_stopwatch \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":201}' 2>/dev/null
```

```output
  Stopwatch started
  Error: start issue stopwatch err: 400 Bad Request — stopwatch already running on this issue
```

Forgejo enforces one stopwatch per issue. The tool description flags this so the agent can recover (e.g. by stopping or cancelling first).

### Step 9: Clean up

```bash
./forgejo-mcp --cli cancel_issue_stopwatch \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":201}' 2>/dev/null

./forgejo-mcp --cli issue_state_change \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":201,"state":"closed"}' 2>/dev/null
```

## Notes

- **Stop vs cancel**: `stop` writes a `TrackedTime` entry; `cancel` does not. Use `cancel` for false starts or when the elapsed time is noise (e.g. a test run).
- **Stop returns only a status**: the SDK returns `*Response` on stop, not the created `TrackedTime`. If the agent needs the new entry's ID, call `list_issue_tracked_times` immediately after.
- **Ledger companion**: see [`demos/issue-time-tracking.md`](issue-time-tracking.md) for manual CRUD on tracked-time entries.
