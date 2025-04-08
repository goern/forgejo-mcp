export const validUserData = {
  id: 56207,
  login: "goern",
  login_name: "",
  source_id: 0,
  full_name: "Christoph Görn",
  email: "christoph@goern.name",
  avatar_url:
    "https://codeberg.org/avatars/4771738d0ef74ceeef0ea68f02b59f0760d3aca3f496073fc8cb12ab7871c80a",
  html_url: "https://codeberg.org/goern",
  language: "de-DE",
  is_admin: false,
  last_login: "2024-12-13T20:03:49Z",
  created: "2022-07-04T04:28:06Z",
  restricted: false,
  active: true,
  prohibit_login: false,
  location: "",
  pronouns: "",
  website: "https://bonn.social/@goern",
  description: "",
  visibility: "public",
  followers_count: 0,
  following_count: 1,
  starred_repos_count: 5,
  username: "goern",
};

export const validRepoData = {
  id: 399408,
  owner: validUserData,
  name: "mcp-codeberg",
  full_name: "goern/mcp-codeberg",
  description:
    "This is a Model Context Protocol (MCP) server that provides tools and resources for interacting with the Codeberg.org REST API.",
  empty: false,
  private: false,
  fork: false,
  template: false,
  parent: null,
  mirror: false,
  size: 819,
  language: "JavaScript",
  languages_url:
    "https://codeberg.org/api/v1/repos/goern/mcp-codeberg/languages",
  html_url: "https://codeberg.org/goern/mcp-codeberg",
  url: "https://codeberg.org/api/v1/repos/goern/mcp-codeberg",
  link: "",
  ssh_url: "git@codeberg.org:goern/mcp-codeberg.git",
  clone_url: "https://codeberg.org/goern/mcp-codeberg.git",
  original_url: "",
  website: "",
  stars_count: 0,
  forks_count: 0,
  watchers_count: 1,
  open_issues_count: 0,
  open_pr_counter: 0,
  release_counter: 0,
  default_branch: "main",
  archived: false,
  created_at: "2025-03-30T15:00:13Z",
  updated_at: "2025-04-06T15:04:40Z",
  archived_at: "1970-01-01T00:00:00Z",
  permissions: {
    admin: true,
    push: true,
    pull: true,
  },
  has_issues: true,
  internal_tracker: {
    enable_time_tracker: true,
    allow_only_contributors_to_track_time: true,
    enable_issue_dependencies: true,
  },
  has_wiki: false,
  wiki_branch: "main",
  globally_editable_wiki: false,
  has_pull_requests: true,
  has_projects: false,
  has_releases: false,
  has_packages: false,
  has_actions: false,
  ignore_whitespace_conflicts: false,
  allow_merge_commits: true,
  allow_rebase: true,
  allow_rebase_explicit: true,
  allow_squash_merge: true,
  allow_fast_forward_only_merge: true,
  allow_rebase_update: true,
  default_delete_branch_after_merge: false,
  default_merge_style: "merge",
  default_allow_maintainer_edit: false,
  default_update_style: "merge",
  avatar_url: "",
  internal: false,
  mirror_interval: "",
  object_format_name: "sha1",
  mirror_updated: "0001-01-01T00:00:00Z",
  repo_transfer: null,
  topics: null,
};

export const { owner: _, ...repoDataMissingOwner } = validRepoData;

export const validIssuesData = [
  {
    id: 1247357,
    url: "https://codeberg.org/api/v1/repos/goern/mcp-codeberg/issues/4",
    html_url: "https://codeberg.org/goern/mcp-codeberg/issues/4",
    number: 4,
    user: validUserData,
    original_author: "",
    original_author_id: 0,
    title: "BUG-001: Missing Test Coverage in Core Services",
    body: "## Description\nSeveral critical service methods lack test coverage, which could lead to undiscovered bugs in production. Additionally, some edge cases in error handling and logging are not fully tested.\n\n## Areas Affected\n1. CodebergService\n2. ErrorHandler\n3. Logger\n\n## Specific Issues\n\n### CodebergService\n1. Missing test coverage for:\n   - updateIssue method\n   - getRepository method\n   - listIssues method\n2. Limited test coverage for error handling and retry logic\n3. Edge cases for API responses not fully tested\n\n### ErrorHandler\n1. Edge cases for complex error scenarios not fully tested\n2. Limited testing of error chaining and context preservation\n\n### Logger\n1. Missing tests for custom error type formatting\n2. Limited testing of complex nested context objects\n\n## Priority\nMedium - While the core functionality is tested, missing coverage could hide potential issues.\n\nFor full details, see: /project/issues/bugs/BUG-001_missing_test_coverage/bug_report.md",
    ref: "",
    assets: [],
    labels: [],
    milestone: null,
    assignee: validUserData,
    assignees: [validUserData],
    state: "closed",
    is_locked: false,
    comments: 1,
    created_at: "2025-04-06T12:29:29Z",
    updated_at: "2025-04-06T12:58:24Z",
    closed_at: "2025-04-06T12:58:19Z",
    due_date: null,
    pull_request: null,
    repository: {
      id: 399408,
      name: "mcp-codeberg",
      owner: "goern",
      full_name: "goern/mcp-codeberg",
    },
    pin_order: 0,
  },
];

export const validIssueData = validIssuesData[0];
