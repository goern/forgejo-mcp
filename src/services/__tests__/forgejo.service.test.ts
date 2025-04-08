import { describe, expect, it, jest, beforeEach } from "@jest/globals";
import { ForgejoService } from "../forgejo.service.js";
import { ErrorHandler } from "../error-handler.service.js";
import { Logger } from "../logger.service.js";
import {
  ForgejoConfig,
  ICacheManager,
  IssueState,
  type User,
  type Issue,
  type Repository,
} from "../types.js";

describe("ForgejoService", () => {
  let service: ForgejoService;
  let errorHandler: ErrorHandler;
  let logger: Logger;
  let config: ForgejoConfig;
  let cacheManager: jest.Mocked<ICacheManager>;

  // Mock user data that satisfies the User interface
  const mockUser: User = {
    id: 1,
    login: "owner",
    fullName: "Test Owner",
    email: "owner@example.com",
    avatarUrl: "https://codeberg.org/avatar/1",
    htmlUrl: "https://codeberg.org/owner",
    createdAt: new Date("2025-01-01"),
  };

  beforeEach(() => {
    jest.clearAllMocks();

    config = {
      baseUrl: "https://api.codeberg.org",
      token: "test-token",
      timeout: 5000,
      maxRetries: 3,
    };

    logger = new Logger("TestService");
    errorHandler = new ErrorHandler();

    // Create mock cache manager
    const mockGet = jest.fn() as jest.MockedFunction<ICacheManager["get"]>;
    const mockSet = jest.fn() as jest.MockedFunction<ICacheManager["set"]>;
    const mockDelete = jest.fn() as jest.MockedFunction<
      ICacheManager["delete"]
    >;
    const mockClear = jest.fn() as jest.MockedFunction<ICacheManager["clear"]>;

    mockGet.mockResolvedValue(undefined);
    mockSet.mockResolvedValue();
    mockDelete.mockResolvedValue();
    mockClear.mockResolvedValue();

    cacheManager = {
      get: mockGet,
      set: mockSet,
      delete: mockDelete,
      clear: mockClear,
    };

    service = new ForgejoService(config, errorHandler, logger, cacheManager);
  });

  describe("Service Orchestration", () => {
    it("should delegate repository operations to RepositoryService", async () => {
      const mockRepo: Repository = {
        id: 1,
        name: "test-repo",
        fullName: "owner/test-repo",
        description: "Test repository",
        htmlUrl: "https://codeberg.org/owner/test-repo",
        cloneUrl: "https://codeberg.org/owner/test-repo.git",
        createdAt: new Date("2025-01-01"),
        updatedAt: new Date("2025-01-02"),
        owner: mockUser,
      };

      // Mock the internal repository service methods
      const spy = jest.spyOn(service["repositoryService"], "getRepository");
      spy.mockResolvedValueOnce(mockRepo);

      const result = await service.getRepository("owner", "test-repo");

      expect(spy).toHaveBeenCalledWith("owner", "test-repo");
      expect(result).toEqual(mockRepo);
    });

    it("should delegate issue operations to IssueService", async () => {
      const mockIssue: Issue = {
        id: 1,
        number: 1,
        title: "Test Issue",
        body: "Issue body",
        state: IssueState.Open,
        htmlUrl: "https://codeberg.org/owner/repo/issues/1",
        createdAt: new Date("2025-01-01"),
        updatedAt: new Date("2025-01-02"),
        user: mockUser,
        labels: [],
        assignees: [],
        milestone: undefined,
        comments: 0,
        locked: false,
        lastModifiedBy: undefined,
        lastUpdated: new Date("2025-01-02"),
        updateInProgress: false,
        updateError: undefined,
        validationRules: [],
      };

      // Mock the internal issue service methods
      const spy = jest.spyOn(service["issueService"], "getIssue");
      spy.mockResolvedValueOnce(mockIssue);

      const result = await service.getIssue("owner", "repo", 1);

      expect(spy).toHaveBeenCalledWith("owner", "repo", 1, undefined);
      expect(result).toEqual(mockIssue);
    });

    it("should delegate user operations to UserService", async () => {
      // Mock the internal user service methods
      const spy = jest.spyOn(service["userService"], "getUser");
      spy.mockResolvedValueOnce(mockUser);

      const result = await service.getUser("testuser");

      expect(spy).toHaveBeenCalledWith("testuser");
      expect(result).toEqual(mockUser);
    });
  });
});
