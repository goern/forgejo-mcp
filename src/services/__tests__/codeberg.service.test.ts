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
  IssueState,
  ValidationError,
} from "../types.js";

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
    service = new CodebergService(config, errorHandler, logger);
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
      it("should get issue successfully", async () => {
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
        };

        mockAxios.get.mockResolvedValueOnce(createMockResponse(mockIssue));

        const result = await service.getIssue("owner", "repo", 1);

        expect(mockAxios.get).toHaveBeenCalledWith(
          "/repos/owner/repo/issues/1",
        );
        expect(result.title).toBe("Test Issue");
        expect(result.state).toBe(IssueState.Open);
      });

      it("should throw ValidationError for invalid issue number", async () => {
        await expect(service.getIssue("owner", "repo", 0)).rejects.toThrow(
          ValidationError,
        );
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
