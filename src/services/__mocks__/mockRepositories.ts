export const mockRepos = [
  {
    id: 1,
    name: "repo1",
    full_name: "owner/repo1",
    description: "Test repo 1",
    html_url: "https://codeberg.org/owner/repo1",
    clone_url: "https://codeberg.org/owner/repo1.git",
    created_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-01-02T00:00:00Z",
    owner: {
      id: 1,
      login: "owner",
      avatar_url: "https://codeberg.org/avatar/1",
      html_url: "https://codeberg.org/owner",
    },
  },
];

export const mockRepo = {
  id: 1,
  name: "test-repo",
  full_name: "owner/test-repo",
  description: "Test repository",
  html_url: "https://codeberg.org/owner/test-repo",
  clone_url: "https://codeberg.org/owner/test-repo.git",
  created_at: "2025-01-01T00:00:00Z",
  updated_at: "2025-01-02T00:00:00Z",
  owner: {
    id: 1,
    login: "owner",
    avatar_url: "https://codeberg.org/avatar/1",
    html_url: "https://codeberg.org/owner",
  },
};
