#!/usr/bin/env python3
"""
Fetch notifications from Codeberg (via forgejo-mcp) and GitHub (via gh api)
and display them in a unified markdown view, sorted by newest first.

Usage:
  python3 fetch_notifications.py              # Show all notifications
  python3 fetch_notifications.py next-action  # Show top 5 actionable items
"""

import json
import subprocess
import sys
from datetime import datetime
from typing import Any, Dict, List, Tuple


def run_command(cmd: List[str]) -> str:
    """Run a shell command and return its output."""
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, check=True)
        return result.stdout.strip()
    except subprocess.CalledProcessError as e:
        print(f"Error running {' '.join(cmd)}: {e.stderr}", file=sys.stderr)
        return ""


def fetch_codeberg_notifications() -> List[Dict[str, Any]]:
    """Fetch Codeberg notifications via forgejo-mcp CLI."""
    output = run_command(["forgejo-mcp", "--cli", "check_notifications"])
    if not output:
        return []

    try:
        # Parse the JSON array from the output
        data = json.loads(output)
        notifications = []

        for item in data:
            if item.get("type") == "text":
                result = json.loads(item.get("text", "{}"))
                if "Result" in result:
                    for notif in result["Result"]:
                        repo = notif.get("repository", {})
                        subject = notif.get("subject", {})
                        notifications.append({
                            "source": "codeberg",
                            "id": notif.get("id"),
                            "repo": repo.get("full_name", "unknown"),
                            "title": subject.get("title", "Untitled"),
                            "type": subject.get("type", "Unknown"),
                            "url": notif.get("html_url", ""),
                            "updated_at": notif.get("updated_at", ""),
                        })
        return notifications
    except (json.JSONDecodeError, KeyError) as e:
        print(f"Error parsing Codeberg notifications: {e}", file=sys.stderr)
        return []


def fetch_github_notifications() -> List[Dict[str, Any]]:
    """Fetch GitHub notifications via gh api."""
    output = run_command(["gh", "api", "notifications", "--paginate"])
    if not output:
        return []

    try:
        data = json.loads(output)
        notifications = []

        for notif in data:
            repo = notif.get("repository", {})
            subject = notif.get("subject", {})
            notifications.append({
                "source": "github",
                "id": notif.get("id"),
                "repo": repo.get("full_name", "unknown"),
                "title": subject.get("title", "Untitled"),
                "type": subject.get("type", "Unknown"),
                "url": subject.get("url", ""),
                "updated_at": notif.get("updated_at", ""),
            })
        return notifications
    except json.JSONDecodeError as e:
        print(f"Error parsing GitHub notifications: {e}", file=sys.stderr)
        return []


def is_dependency_update(title: str) -> bool:
    """Check if notification is a pure dependency update."""
    patterns = [
        "chore(deps):", "fix(deps):", "chore: update",
        "update dependency", "pin dependencies",
        "renovate", "dependabot",
    ]
    title_lower = title.lower()
    return any(p in title_lower for p in patterns)


def score_notification(notif: Dict[str, Any], important_repos: List[str]) -> Tuple[int, str]:
    """Score notification for priority. Returns (score, reason)."""
    score = 0
    reasons = []

    title = notif.get("title", "").lower()
    notif_type = notif.get("type", "")
    repo = notif.get("repo", "")

    # Exclude pure dependency updates (low priority)
    if is_dependency_update(notif.get("title", "")):
        return -999, "Dependency update"

    # PRs are more actionable than issues
    if notif_type in ("PullRequest", "Pull"):
        score += 100
        reasons.append("PR")
    elif notif_type in ("Issue", "Issues"):
        score += 50
        reasons.append("Issue")

    # Important repos get higher priority
    if any(imp_repo in repo for imp_repo in important_repos):
        score += 80
        reasons.append(f"Key repo: {repo}")

    # Feature/fix issues are more important than research
    if any(x in title for x in ["feat:", "fix:", "refactor:"]):
        score += 40
        reasons.append("Feature/Fix")

    # Exclude research issues (lower priority)
    if "research:" in title:
        score -= 50
        reasons.append("Research")

    return score, " | ".join(reasons)


def get_next_actions(notifications: List[Dict[str, Any]], limit: int = 5) -> List[Dict[str, Any]]:
    """Filter and prioritize notifications for next actions."""
    # Score all notifications
    scored = []
    for notif in notifications:
        important_repos = ["b4arena", "forgejo-mcp", "ludus", "infra"]
        score, reason = score_notification(notif, important_repos)
        notif["_score"] = score
        notif["_reason"] = reason
        if score >= 0:  # Only include actionable items
            scored.append(notif)

    # Sort by score (descending), then by date
    scored.sort(
        key=lambda x: (
            -x.get("_score", 0),
            -datetime.fromisoformat(x.get("updated_at", "0000-00-00T00:00:00Z").replace("Z", "+00:00")).timestamp()
        )
    )

    return scored[:limit]


def format_notifications(notifications: List[Dict[str, Any]], mode: str = "all") -> str:
    """Format notifications as markdown for user-friendly display."""
    if not notifications:
        return "## 📬 No notifications\n"

    # Sort by updated_at, newest first (handle missing dates)
    notifications.sort(
        key=lambda x: x.get("updated_at", "0000-00-00T00:00:00Z"),
        reverse=True
    )

    # Filter for next-action mode
    if mode == "next-action":
        notifications = get_next_actions(notifications, limit=5)
        header = "⚡ **Next Actions — Top 5**"
        subtitle = "Items that need your attention"
    else:
        header = "📬 **All Notifications**"
        subtitle = "Everything across GitHub and Codeberg"

    # Group by source
    github_notifs = [n for n in notifications if n["source"] == "github"]
    codeberg_notifs = [n for n in notifications if n["source"] == "codeberg"]

    output = []
    output.append(f"# {header}\n")
    output.append(f"_{subtitle}_\n")

    # Count summary with badges
    total = len(notifications)
    gh_count = len(github_notifs)
    cb_count = len(codeberg_notifs)
    output.append(f"**{total} notifications** — GitHub {gh_count} · Codeberg {cb_count}\n\n")

    # GitHub notifications
    if github_notifs:
        output.append("## 🐙 GitHub\n")
        for idx, notif in enumerate(github_notifs, 1):
            type_icon = "🔀" if notif["type"] == "PullRequest" else "📌"
            type_label = "PR" if notif["type"] == "PullRequest" else "Issue"

            # Format: number. icon TYPE | repo | title
            output.append(f"{idx}. {type_icon} **{type_label}** | ")
            output.append(f"`{notif['repo']}` | ")
            output.append(f"{notif['title']}\n")
            output.append(f"   {notif['url']}\n")

            if mode == "next-action":
                reason = notif.get('_reason', '')
                if reason:
                    output.append(f"   _{reason}_\n")
        output.append("\n")

    # Codeberg notifications
    if codeberg_notifs:
        output.append("## 🦊 Codeberg\n")
        start_idx = len(github_notifs) + 1
        for idx, notif in enumerate(codeberg_notifs, start_idx):
            type_icon = "🔀" if notif["type"] == "Pull" else "📌"
            type_label = "MR" if notif["type"] == "Pull" else "Issue"

            # Format: number. icon TYPE | repo | title
            output.append(f"{idx}. {type_icon} **{type_label}** | ")
            output.append(f"`{notif['repo']}` | ")
            output.append(f"{notif['title']}\n")
            output.append(f"   {notif['url']}\n")

            if mode == "next-action":
                reason = notif.get('_reason', '')
                if reason:
                    output.append(f"   _{reason}_\n")

    return "".join(output)


def main():
    """Fetch and display unified notifications."""
    print("Fetching notifications...\n", file=sys.stderr)

    # Check for mode argument
    mode = "all"
    if len(sys.argv) > 1:
        arg = sys.argv[1]
        # Accept both singular and plural variants
        if arg in ("all",):
            mode = "all"
        elif arg in ("next-action", "next-actions"):
            mode = "next-action"
        else:
            print(f"Unknown mode: {arg}. Use 'all', 'next-action', or 'next-actions'.", file=sys.stderr)
            sys.exit(1)

    codeberg = fetch_codeberg_notifications()
    github = fetch_github_notifications()

    all_notifications = codeberg + github
    markdown = format_notifications(all_notifications, mode=mode)

    print(markdown)


if __name__ == "__main__":
    main()
