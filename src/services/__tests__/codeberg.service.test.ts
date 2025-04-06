import { describe, expect, it, jest, beforeEach } from "@jest/globals";
import axios, {
  AxiosError,
  AxiosInstance,
  AxiosResponse,
  InternalAxiosRequestConfig,
  RawAxiosRequestHeaders,
} from "axios";
import { CodebergService } from "../codeberg.service.js";
import { ErrorHandler } from "../error-handler.service.js";
import { Logger } from "../logger.service.js";
import {
  ApiError,
  CodebergConfig,
  ICacheManager,
  IssueState,
  ValidationError,
} from "../types.js";
import { MockCacheManager } from "./mock-cache-manager.js";

// Mock axios module before creating mock instance
jest.mock("axios");

// Create mock instance
const mockAxios = {
  get: jest.fn(),
  post: jest.fn(),
  patch: jest.fn(),
  delete: jest.fn(),
  defaults: {},
} as unknown as jest.Mocked<AxiosInstance>;

// Configure axios mock
(axios as unknown as { create: jest.Mock }).create = jest.fn(() => mockAxios);
(axios as unknown as { isAxiosError: jest.Mock }).isAxiosError = jest.fn(
  (error: unknown) => error instanceof AxiosError,
);

// Helper to create mock response
const createMockResponse = <T>(data: T, status = 200): AxiosResponse<T> => ({
  data,
  status,
  statusText: status === 200 ? "OK" : "Error",
  headers: {},
  config: {
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
    } as RawAxiosRequestHeaders,
  } as InternalAxiosRequestConfig,
});

describe("CodebergService", () => {
  let service: CodebergService;
  let errorHandler: ErrorHandler;
  let logger: Logger;
  let config: CodebergConfig;
  let cacheManager: jest.Mocked<ICacheManager>;

  beforeEach(() => {
    // Reset mocks
    jest.clearAllMocks();

    // Setup test configuration
    config = {
      baseUrl: "https://api.codeberg.org",
      token: "test-token",
      timeout: 5000,
      maxRetries: 3,
    };

    // Create service instances
    logger = new Logger("TestService");
    errorHandler = new ErrorHandler();
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
    } as jest.Mocked<ICacheManager>;
    service = new CodebergService(config, errorHandler, logger, cacheManager);
  });

  describe("Repository Operations", () => {
    describe("listRepositories", () => {
      it("should list repositories successfully", async () => {
        const mockRepos = [
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

        mockAxios.get.mockResolvedValueOnce(createMockResponse(mockRepos));

        const result = await service.listRepositories("owner");

        expect(mockAxios.get).toHaveBeenCalledWith("/users/owner/repos");
        expect(result).toHaveLength(1);
        expect(result[0].name).toBe("repo1");
      });

      it("should throw ValidationError for empty owner", async () => {
        await expect(service.listRepositories("")).rejects.toThrow(
          ValidationError,
        );
      });

      it("should handle API errors", async () => {
        const error = new AxiosError();
        error.response = createMockResponse({ message: "User not found" }, 404);

        mockAxios.get.mockRejectedValueOnce(error);

        await expect(service.listRepositories("owner")).rejects.toThrow(
          ApiError,
        );
      });
    });

    describe("getRepository", () => {
      it("should get repository successfully", async () => {
        const mockRepo = {
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

        mockAxios.get.mockResolvedValueOnce(createMockResponse(mockRepo));

        const result = await service.getRepository("owner", "test-repo");

        expect(mockAxios.get).toHaveBeenCalledWith("/repos/owner/test-repo");
        expect(result.name).toBe("test-repo");
        expect(result.fullName).toBe("owner/test-repo");
      });

      it("should throw ValidationError for empty repository name", async () => {
        await expect(service.getRepository("owner", "")).rejects.toThrow(
          ValidationError,
        );
      });

      it("should handle repository not found", async () => {
        const error = new AxiosError();
        error.response = createMockResponse(
          { message: "Repository not found" },
          404,
        );

        mockAxios.get.mockRejectedValueOnce(error);

        await expect(
          service.getRepository("owner", "non-existent"),
        ).rejects.toThrow(ApiError);
      });
    });
  });

  describe("Issue Operations", () => {
    describe("listIssues", () => {
      it("should list issues successfully", async () => {
        const mockIssues = [
          {
            id: 1,
            number: 1,
            title: "First Issue",
            body: "Issue body",
            state: "open",
            html_url: "https://codeberg.org/owner/repo/issues/1",
            created_at: "2025-01-01T00:00:00Z",
            updated_at: "2025-01-02T00:00:00Z",
            user: {
              id: 1,
              login: "user",
              avatar_url: "https://codeberg.org/avatar/1",
              html_url: "https://codeberg.org/user",
            },
            labels: [],
          },
          {
            id: 2,
            number: 2,
            title: "Second Issue",
            body: "Another issue",
            state: "closed",
            html_url: "https://codeberg.org/owner/repo/issues/2",
            created_at: "2025-01-03T00:00:00Z",
            updated_at: "2025-01-04T00:00:00Z",
            user: {
              id: 1,
              login: "user",
              avatar_url: "https://codeberg.org/avatar/1",
              html_url: "https://codeberg.org/user",
            },
            labels: [],
          },
        ];

        mockAxios.get.mockResolvedValueOnce(createMockResponse(mockIssues));

        const result = await service.listIssues("owner", "repo", {
          state: IssueState.All,
        });

        expect(mockAxios.get).toHaveBeenCalledWith("/repos/owner/repo/issues", {
          params: { state: IssueState.All },
        });
        expect(result).toHaveLength(2);
        expect(result[0].title).toBe("First Issue");
        expect(result[1].state).toBe(IssueState.Closed);
      });

      it("should handle empty result", async () => {
        mockAxios.get.mockResolvedValueOnce(createMockResponse([]));

        const result = await service.listIssues("owner", "repo");

        expect(result).toHaveLength(0);
      });

      it("should throw ValidationError for invalid repository", async () => {
        await expect(service.listIssues("", "repo")).rejects.toThrow(
          ValidationError,
        );
      });

      it("should handle API errors", async () => {
        const error = new AxiosError();
        error.response = createMockResponse(
          { message: "Repository not found" },
          404,
        );

        mockAxios.get.mockRejectedValueOnce(error);

        await expect(service.listIssues("owner", "repo")).rejects.toThrow(
          ApiError,
        );
      });
    });

    describe("getIssue", () => {
      const mockIssue = {
        id: 1,
        number: 1,
        title: "Test Issue",
        body: "Issue body",
        state: "open",
        html_url: "https://codeberg.org/owner/repo/issues/1",
        created_at: "2025-01-01T00:00:00Z",
        updated_at: "2025-01-02T00:00:00Z",
        user: {
          id: 1,
          login: "user",
          avatar_url: "https://codeberg.org/avatar/1",
          html_url: "https://codeberg.org/user",
        },
        labels: [],
        assignees: [],
        milestone: null,
        comments: 0,
        locked: false,
      };

      const mockComments = [
        { id: 1, body: "Comment 1" },
        { id: 2, body: "Comment 2" },
      ];

      const mockEvents = [
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

      const mockMilestone = {
        id: 1,
        number: 1,
        title: "v1.0",
        description: "First release",
        due_on: "2025-02-01T00:00:00Z",
        state: "open",
        created_at: "2025-01-01T00:00:00Z",
        updated_at: "2025-01-01T00:00:00Z",
      };

      it("should get issue successfully with all metadata", async () => {
        mockAxios.get
          .mockResolvedValueOnce(createMockResponse(mockIssue))
          .mockResolvedValueOnce(createMockResponse(mockComments))
          .mockResolvedValueOnce(createMockResponse(mockEvents))
          .mockResolvedValueOnce(createMockResponse(mockMilestone));

        const result = await service.getIssue("owner", "repo", 1, {
          includeMetadata: true,
        });

        expect(mockAxios.get).toHaveBeenCalledWith(
          "/repos/owner/repo/issues/1",
        );
        expect(mockAxios.get).toHaveBeenCalledWith(
          "/repos/owner/repo/issues/1/comments",
        );
        expect(mockAxios.get).toHaveBeenCalledWith(
          "/repos/owner/repo/issues/1/events",
        );
        expect(mockAxios.get).toHaveBeenCalledWith(
          "/repos/owner/repo/issues/1/milestone",
        );

        expect(result.title).toBe("Test Issue");
        expect(result.state).toBe(IssueState.Open);
        expect(result.comments).toBe(2);
        expect(result.lastModifiedBy?.login).toBe("modifier");
        expect(result.milestone).toBeDefined();
        expect(result.milestone?.title).toBe("v1.0");
        expect(result.validationRules).toHaveLength(2);
        expect(result.validationRules[0]).toEqual({
          field: "title",
          type: "required",
          message: "Issue title is required",
        });
      });

      it("should return cached issue when available and valid", async () => {
        const cachedIssue = {
          ...mockIssue,
          title: "Cached Issue",
          validationRules: [],
          assignees: [],
          lastUpdated: new Date(),
          updateInProgress: false,
        };

        cacheManager.get.mockResolvedValueOnce(cachedIssue);

        const result = await service.getIssue("owner", "repo", 1);

        expect(cacheManager.get).toHaveBeenCalledWith("issue:owner:repo:1");
        expect(mockAxios.get).not.toHaveBeenCalled();
        expect(result.title).toBe("Cached Issue");
      });

      it("should invalidate and refetch when cached data is invalid", async () => {
        const invalidCachedIssue = {
          id: 1,
          title: "Invalid Issue",
          // Missing required fields
        };

        cacheManager.get.mockResolvedValueOnce(invalidCachedIssue);
        mockAxios.get.mockResolvedValueOnce(createMockResponse(mockIssue));

        const result = await service.getIssue("owner", "repo", 1);

        expect(cacheManager.delete).toHaveBeenCalledWith("issue:owner:repo:1");
        expect(mockAxios.get).toHaveBeenCalledWith(
          "/repos/owner/repo/issues/1",
        );
        expect(result.title).toBe("Test Issue");
      });

      it("should force fresh data when requested", async () => {
        mockAxios.get.mockResolvedValueOnce(createMockResponse(mockIssue));

        await service.getIssue("owner", "repo", 1, { forceFresh: true });

        expect(cacheManager.get).not.toHaveBeenCalled();
        expect(mockAxios.get).toHaveBeenCalledWith(
          "/repos/owner/repo/issues/1",
        );
      });

      it("should cache successful responses with state-based TTL", async () => {
        const closedIssue = { ...mockIssue, state: "closed" };
        mockAxios.get.mockResolvedValueOnce(createMockResponse(closedIssue));

        await service.getIssue("owner", "repo", 1);

        expect(cacheManager.set).toHaveBeenCalledWith(
          "issue:owner:repo:1",
          expect.objectContaining({ id: 1, state: "closed" }),
          3600, // 1 hour TTL for closed issues
        );

        mockAxios.get.mockResolvedValueOnce(createMockResponse(mockIssue));

        await service.getIssue("owner", "repo", 2);

        expect(cacheManager.set).toHaveBeenCalledWith(
          "issue:owner:repo:2",
          expect.objectContaining({ id: 1, state: "open" }),
          300, // 5 minutes TTL for open issues
        );
      });

      it("should throw ValidationError for invalid issue number", async () => {
        await expect(service.getIssue("owner", "repo", 0)).rejects.toThrow(
          ValidationError,
        );
      });

      it("should handle metadata fetch failures gracefully", async () => {
        mockAxios.get
          .mockResolvedValueOnce(createMockResponse(mockIssue))
          .mockRejectedValueOnce(new Error("Failed to fetch comments"))
          .mockRejectedValueOnce(new Error("Failed to fetch events"))
          .mockRejectedValueOnce(new Error("Failed to fetch milestone"));

        const result = await service.getIssue("owner", "repo", 1, {
          includeMetadata: true,
        });

        expect(result.title).toBe("Test Issue");
        expect(result.comments).toBe(0);
        expect(result.lastModifiedBy).toBeUndefined();
        expect(result.milestone).toBeUndefined();
        expect(result.validationRules).toBeDefined();
        expect(result.validationRules).toHaveLength(2);
      });
    });

    describe("createIssue", () => {
      it("should create issue successfully", async () => {
        const mockIssue = {
          id: 1,
          number: 1,
          title: "New Issue",
          body: "Issue body",
          state: "open",
          html_url: "https://codeberg.org/owner/repo/issues/1",
          created_at: "2025-01-01T00:00:00Z",
          updated_at: "2025-01-01T00:00:00Z",
          user: {
            id: 1,
            login: "user",
            avatar_url: "https://codeberg.org/avatar/1",
            html_url: "https://codeberg.org/user",
          },
          labels: [],
        };

        mockAxios.post.mockResolvedValueOnce(
          createMockResponse(mockIssue, 201),
        );

        const result = await service.createIssue("owner", "repo", {
          title: "New Issue",
          body: "Issue body",
        });

        expect(mockAxios.post).toHaveBeenCalledWith(
          "/repos/owner/repo/issues",
          {
            title: "New Issue",
            body: "Issue body",
          },
        );
        expect(result.title).toBe("New Issue");
      });

      it("should throw ValidationError for empty title", async () => {
        await expect(
          service.createIssue("owner", "repo", { title: "", body: "body" }),
        ).rejects.toThrow(ValidationError);
      });
    });

    describe("updateIssue", () => {
      it("should update issue successfully", async () => {
        const mockIssue = {
          id: 1,
          number: 1,
          title: "Updated Issue",
          body: "Updated body",
          state: "closed",
          html_url: "https://codeberg.org/owner/repo/issues/1",
          created_at: "2025-01-01T00:00:00Z",
          updated_at: "2025-01-02T00:00:00Z",
          user: {
            id: 1,
            login: "user",
            avatar_url: "https://codeberg.org/avatar/1",
            html_url: "https://codeberg.org/user",
          },
          labels: [],
        };

        mockAxios.patch.mockResolvedValueOnce(createMockResponse(mockIssue));

        const result = await service.updateIssue("owner", "repo", 1, {
          title: "Updated Issue",
          body: "Updated body",
          state: IssueState.Closed,
        });

        expect(mockAxios.patch).toHaveBeenCalledWith(
          "/repos/owner/repo/issues/1",
          {
            title: "Updated Issue",
            body: "Updated body",
            state: IssueState.Closed,
          },
        );
        expect(result.title).toBe("Updated Issue");
        expect(result.state).toBe(IssueState.Closed);
      });

      it("should throw ValidationError for invalid issue number", async () => {
        await expect(
          service.updateIssue("owner", "repo", 0, { title: "Updated" }),
        ).rejects.toThrow(ValidationError);
      });

      it("should handle API errors", async () => {
        const error = new AxiosError();
        error.response = createMockResponse(
          { message: "Issue not found" },
          404,
        );

        mockAxios.patch.mockRejectedValueOnce(error);

        await expect(
          service.updateIssue("owner", "repo", 1, { title: "Updated" }),
        ).rejects.toThrow(ApiError);
      });
    });
  });
  describe("User Operations", () => {
    describe("getCurrentUser", () => {
      it("should get current user successfully", async () => {
        const mockUser = {
          id: 1,
          login: "user",
          full_name: "Test User",
          email: "user@example.com",
          avatar_url: "https://codeberg.org/avatar/1",
          html_url: "https://codeberg.org/user",
          created_at: "2025-01-01T00:00:00Z",
        };

        mockAxios.get.mockResolvedValueOnce(createMockResponse(mockUser));

        const result = await service.getCurrentUser();

        expect(mockAxios.get).toHaveBeenCalledWith("/user");
        expect(result.login).toBe("user");
        expect(result.fullName).toBe("Test User");
      });
    });
  });
});
