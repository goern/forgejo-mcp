# Demo: notifications-management

*2026-03-08T09:35:00Z by Showboat 0.6.1*
<!-- showboat-id: e4f2327f-53ee-432b-8971-2f9aae4650d8 -->

## What these tools do

The new notification management tools expand the Forgejo-MCP server to cover 100% of the notification API. They allow autonomous agents to:
- Retrieve individual notification threads (`get_notification_thread`)
- Mark notifications as read (`mark_notification_read`)
- Acknowledge all notifications at once (`mark_all_notifications_read`)
- Filter notifications scoped to a single repository (`list_repo_notifications`)
- Mark all notifications in a specific repo as read (`mark_repo_notifications_read`)

This completes the autonomous feedback loop without relying on web GUIs.

## Setup

Set FORGEJO_URL and FORGEJO_ACCESS_TOKEN environment variables, then invoke via the CLI.

```bash
FORGEJO_URL=https://codeberg.org FORGEJO_ACCESS_TOKEN=dummy_token_12345 forgejo-mcp --cli list_repo_notifications --help 2>/dev/null
```

```output
Tool: list_repo_notifications
Description: Filter notifications scoped to a single repository

Parameters:
  all                  boolean    optional   Include read notifications (default: false)
  before               string     optional   Before time (RFC3339)
  limit                number     optional   Page size
  owner                string     required   Repository owner
  page                 number     optional   Page number (1-based)
  repo                 string     required   Repository name
  since                string     optional   After time (RFC3339)
```

```bash
FORGEJO_URL=https://codeberg.org FORGEJO_ACCESS_TOKEN=dummy_token_12345 forgejo-mcp --cli get_notification_thread --help 2>/dev/null
```

```output
Tool: get_notification_thread
Description: Get detailed info on a single notification thread

Parameters:
  id                   number     required   Notification ID
```
