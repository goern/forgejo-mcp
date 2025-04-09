import type { User } from "../types.js";

export const mockUser: User = {
  id: 1,
  login: "owner",
  fullName: "Test Owner",
  email: "owner@example.com",
  avatarUrl: "https://codeberg.org/avatar/1",
  htmlUrl: "https://codeberg.org/owner",
  createdAt: new Date("2025-01-01"),
};
