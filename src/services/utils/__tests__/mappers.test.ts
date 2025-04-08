import { ForgejoMappers } from "../../utils/mappers.js";
import {
  InvalidRepositoryDataError,
  InvalidUserDataError,
} from "../../types.js";
import {
  validRepoData,
  repoDataMissingOwner,
  validUserData,
  validIssuesData,
  validIssueData,
} from "./fixtures/mappers.fixtures.js";

describe("ForgejoMappers", () => {
  describe("mapRepository", () => {
    it("should map valid repository data correctly", () => {
      const repo = ForgejoMappers.mapRepository(validRepoData);

      expect(repo.id).toBe(399408);
      expect(repo.name).toBe("mcp-codeberg");
      expect(repo.owner.login).toBe("goern");
      expect(repo.owner.fullName).toBe("Christoph Görn");
    });

    it("should throw InvalidRepositoryDataError if data is null", () => {
      expect(() => ForgejoMappers.mapRepository(null)).toThrow(
        InvalidRepositoryDataError,
      );
    });

    it("should throw InvalidRepositoryDataError if owner is missing", () => {
      expect(() => ForgejoMappers.mapRepository(repoDataMissingOwner)).toThrow(
        InvalidRepositoryDataError,
      );
    });
  });

  describe("mapUser", () => {
    it("should map valid user data correctly", () => {
      const user = ForgejoMappers.mapUser(validUserData);

      expect(user.id).toBe(56207);
      expect(user.login).toBe("goern");
      expect(user.fullName).toBe("Christoph Görn");
    });
    it("should throw InvalidUserDataError if data is null", () => {
      expect(() => ForgejoMappers.mapUser(null)).toThrow(InvalidUserDataError);
    });
  });

  describe("ForgejoMappers.mapIssue (array)", () => {
    it("should map an array of issue data correctly", () => {
      const issues = validIssuesData.map(ForgejoMappers.mapIssue);

      expect(Array.isArray(issues)).toBe(true);
      expect(issues.length).toBe(validIssuesData.length);

      const first = issues[0];
      expect(first.id).toBe(1247357);
      expect(first.number).toBe(4);
      expect(first.title).toContain("Missing Test Coverage");
      expect(first.state).toBe("closed");
      expect(first.user.login).toBe("goern");
      expect(first.createdAt).toBeInstanceOf(Date);
      expect(first.updatedAt).toBeInstanceOf(Date);
      expect(first.comments).toBe(1);
      expect(first.locked).toBe(false);
      expect(first.assignees).toBeInstanceOf(Array);
      expect(first.assignees.length).toBeGreaterThanOrEqual(1);
      expect(first.assignees[0].login).toBe("goern");
      expect(first.assignees[0].fullName).toBe("Christoph Görn");
    });
  });

  describe("ForgejoMappers.mapIssue", () => {
    it("should map valid issue data correctly", () => {
      const issue = ForgejoMappers.mapIssue(validIssueData);

      expect(issue.id).toBe(1247357);
      expect(issue.number).toBe(4);
      expect(issue.title).toContain("Missing Test Coverage");
      expect(issue.state).toBe("closed");
      expect(issue.user.login).toBe("goern");
      expect(issue.createdAt).toBeInstanceOf(Date);
      expect(issue.updatedAt).toBeInstanceOf(Date);
      expect(issue.comments).toBe(1);
      expect(issue.locked).toBe(false);
      expect(issue.labels).toBeInstanceOf(Array);
      expect(issue.assignees).toBeInstanceOf(Array);
    });
  });
});
