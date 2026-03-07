# Demo: check_notifications

*2026-03-07T09:19:13Z by Showboat 0.6.1*
<!-- showboat-id: 7b92327f-53ee-432b-8971-2f9aae4650d7 -->

## What this tool does

`check_notifications` retrieves unread (and optionally all) Forgejo notifications for the authenticated user — across all repositories they watch or participate in.

This enables AI agents to monitor activity without polling individual repos.

## Setup

Set FORGEJO_URL and FORGEJO_ACCESS_TOKEN environment variables, then invoke via the CLI.

```bash
FORGEJO_URL=https://codeberg.org FORGEJO_ACCESS_TOKEN=fb39f948512ab94292eb44b11ab542622709ff85 /tmp/bin84 --cli check_notifications --help 2>/dev/null
```

```output
Tool: check_notifications
Description: Check and list user notifications

Parameters:
  all                  boolean    optional   Include read notifications (default: false)
  before               string     optional   Before time (RFC3339)
  limit                number     optional   Page size
  page                 number     optional   Page number (1-based)
  since                string     optional   After time (RFC3339)
```

## Live demo: unread notifications (12 unread)

```bash
FORGEJO_URL=https://codeberg.org FORGEJO_ACCESS_TOKEN=fb39f948512ab94292eb44b11ab542622709ff85 /tmp/bin84 --cli check_notifications --args '{}' 2>/dev/null | python3 /tmp/fmt_notifications.py
```

```output
  ● [Pull  open   ]  goern/forgejo-mcp  —  feat: add list_repo_milestones + list_repo_labels MCP tools 
  ● [Pull  merged ]  llnvd/openclaw-url-guard  —  feat: implement E2E integration tests with real OpenClaw gat
  ● [Issue closed ]  llnvd/openclaw-url-guard  —  End-to-End Tests with Real OpenClaw Instance
  ● [Issue open   ]  goern/forgejo-mcp  —  feat: Add tools for listing Milestones and Labels (`list_rep
  ● [Pull  merged ]  goern/forgejo-mcp  —  Spec: add list_repo_milestones + list_repo_labels (closes #8
  ● [Pull  merged ]  llnvd/openclaw-url-guard  —  spec: OpenSpec for E2E integration tests with real OpenClaw 
  ● [Pull  merged ]  llnvd/openclaw-url-guard  —  feat: Enhance test coverage and improve documentation
  ● [Pull  closed ]  goern/hugo-pages.d  —  post: Security Is the Bottleneck — position paper on securit
  ● [Pull  merged ]  goern/hugo-pages.d  —  ci: add Forgejo workflow to build and deploy Hugo site on me
  ● [Pull  closed ]  goern/forgejo-mcp  —  test: add race condition reproducer for #76
  ● [Pull  merged ]  goern/forgejo-mcp  —  fix: nil pointer deref and flag.Parse() in init() (#76)
  ● [Issue closed ]  goern/forgejo-mcp  —  Bug: fatal error: concurrent map writes
```

An AI agent workflow: call check_notifications to get unread activity, inspect subject.type (Issue/Pull) and subject.state, then use get_issue_by_index or get_pull_request for full details — without any manual polling.
