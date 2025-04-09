import type { Issue, User } from "../types.js";

export const mockUser: User = {
  id: 1,
  login: "user",
  fullName: "Test User",
  email: "user@test.com",
  avatarUrl: "https://codeberg.org/avatar/1",
  htmlUrl: "https://codeberg.org/user",
  createdAt: new Date("2025-01-01T00:00:00Z"),
};

export const mockIssueData = {
  id: 1,
  number: 1,
  title: "Test Issue",
  body: "Issue body",
  state: "open",
  htmlUrl: "https://codeberg.org/owner/repo/issues/1",
  createdAt: new Date("2025-01-01T00:00:00Z"),
  updatedAt: new Date("2025-01-02T00:00:00Z"),
  user: mockUser,
  labels: [],
  assignees: [],
  milestone: null,
  comments: 0,
  locked: false,
};

export const mockComments = [
  { id: 1, body: "Comment 1" },
  { id: 2, body: "Comment 2" },
];

export const mockEvents = [
  {
    id: 1,
    actor: {
      id: 2,
      login: "modifier",
      avatar_url: "https://codeberg.org/avatar/2",
      html_url: "https://codeberg.org/modifier",
    },
  },
];

export const mockMilestone = {
  id: 1,
  number: 1,
  title: "v1.0",
  description: "First release",
  dueDate: new Date("2025-02-01T00:00:00Z"),
  state: "open" as const,
  createdAt: new Date("2025-01-01T00:00:00Z"),
  updatedAt: new Date("2025-01-01T00:00:00Z"),
};
