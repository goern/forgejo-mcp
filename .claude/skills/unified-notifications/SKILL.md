---
name: unified-notifications
description: |
  Fetch and display notifications from both GitHub and Codeberg in a unified markdown view with clickable links.
  Two modes: all (58+ notifications) or next-action (top 5 actionable items). Shows notifications sorted by newest first, grouped by platform.
  Use /unified-notifications for all notifications. Use next-action mode (python3 .claude/skills/unified-notifications/scripts/fetch_notifications.py next-action) to see only the top 5 items needing your attention.
  Triggers on requests like "check notifications", "show my notifications", "unified notifications", "next actions", "what needs attention", or "/unified-notifications".
---

# Unified Notifications

View all your notifications from GitHub and Codeberg in a single markdown view with clickable links, sorted by newest first.

## Quick Start

### All notifications
```bash
python3 scripts/fetch_notifications.py
```

### Top 5 actionable items (next-action mode)
```bash
python3 scripts/fetch_notifications.py next-action
```

Output shows:
- Summary counts (total, GitHub, Codeberg)
- All notifications sorted by newest first
- Grouped by platform
- Icons: 🔀 (PR/MR), 📌 (Issue)
- Direct links to each item
- **In next-action mode**: Prioritization reasons for each item

## Example Output

```
# 📬 Unified Notifications
**Total**: 58 | **GitHub**: 38 | **Codeberg**: 20

## GitHub
- 🔀 **[owner/repo](https://github.com/owner/repo)** [PR Title](link)
- 📌 **[owner/repo](https://github.com/owner/repo)** [Issue Title](link)

## Codeberg
- 🔀 **[owner/repo](https://codeberg.org/owner/repo)** [MR Title](link)
```

## Modes

### All Mode (default)
Shows all 58+ notifications across GitHub and Codeberg, sorted by newest first.

### Next-Action Mode
Intelligently filters to show only the 5 most actionable items:

**Prioritization logic:**
- ✅ Excludes dependency updates (chore(deps), renovate, etc.) - too low priority
- ✅ Prioritizes PRs over issues - more action-oriented
- ✅ Prioritizes key repos (b4arena, forgejo-mcp, ludus, infra)
- ✅ Prioritizes feature/fix items over research
- ✅ Includes a reason for each item's ranking

**Example next-action items:**
- Feature PRs in important repos
- Bug fix PRs
- Critical issues assigned to you
- Items requiring your review/action

## Integration with Claude Code

### Show all notifications (58+ items)
```
/unified-notifications
```

### Show only top 5 actionable items (recommended)
```
python3 .claude/skills/unified-notifications/scripts/fetch_notifications.py next-action
```

Or use the full path:
```
python3 /var/home/goern/.claude/skills/unified-notifications/scripts/fetch_notifications.py next-action
```

**Tip:** For a quick overview of what needs attention, use the next-action command above. For a comprehensive view of all notifications, use `/unified-notifications`.

## Requirements

- `gh` CLI (GitHub) installed and authenticated
- `forgejo-mcp` CLI installed and configured
- Python 3.6+

## Troubleshooting

If notifications don't appear:
- Verify `gh api notifications --paginate` returns data
- Verify `forgejo-mcp --cli check_notifications` returns data
- Check that both tools are properly authenticated
