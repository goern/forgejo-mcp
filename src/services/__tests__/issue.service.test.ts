import { describe, expect, it, jest, beforeEach } from "@jest/globals";
import axios, { AxiosError, AxiosInstance } from "axios";
import { IssueService } from "../issue.service.js";
import { ErrorHandler } from "../error-handler.service.js";
import { Logger } from "../logger.service.js";
import {
  ApiError,
  ICacheManager,
  ValidationError,
  IssueState,
  isIssue,
  isMilestone,
  Issue,
  ForgejoConfig,
} from "../types.js";

// Mock axios module
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

// Mock data
const mockIssueData = {
  id: 1,
  number: 1,
  title: "Test Issue",
  body: "Issue body",
  state: IssueState.Open,
  htmlUrl: "https://codeberg.org/owner/repo/issues/1",
  createdAt: new Date("2025-01-01T00:00:00Z"),
  updatedAt: new Date("2025-01-02T00:00:00Z"),
  user: {
    id: 1,
    login: "user",
    avatarUrl: "https://codeberg.org/avatar/1",
    htmlUrl: "https://codeberg.org/user",
    fullName: "Test User",
    email: "user@test.com",
    createdAt: new Date("2025-01-01T00:00:00Z"),
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
  dueDate: new Date("2025-02-01T00:00:00Z"),
  state: "open" as const,
  createdAt: new Date("2025-01-01T00:00:00Z"),
  updatedAt: new Date("2025-01-01T00:00:00Z"),
};

describe("IssueService", () => {
  let service: IssueService;
  let errorHandler: ErrorHandler;
  let logger: Logger;
  let config: ForgejoConfig;
  let cacheManager: jest.Mocked<ICacheManager>;

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

    service = new IssueService(config, errorHandler, logger, cacheManager);
  });

  describe("listIssues", () => {
    it("should list issues successfully", async () => {
      // Mock data
      const mockIssueDataList = [mockIssueData];

      // Mock the API response with data that matches the mapper's expectations
      mockAxios.get.mockResolvedValueOnce({
        data: mockIssueDataList,
      });

      const result = await service.listIssues("owner", "repo", {
        state: IssueState.All,
      });

      expect(mockAxios.get).toHaveBeenCalledWith("/repos/owner/repo/issues", {
        params: { state: IssueState.All },
      });
      expect(result).toHaveLength(1);
      expect(result[0].title).toBe("Test Issue");
      expect(result[0].state).toBe(IssueState.Open);
    });

    it("should handle empty result", async () => {
      mockAxios.get.mockResolvedValueOnce({ data: [] });
      const result = await service.listIssues("owner", "repo");
      expect(result).toHaveLength(0);
    });

    it("should throw ValidationError for invalid repository", async () => {
      await expect(service.listIssues("", "repo")).rejects.toThrow(
        ValidationError,
      );
    });
  });

  describe("getIssue", () => {
    it("should get issue successfully with all metadata", async () => {
      mockAxios.get
        .mockResolvedValueOnce({ data: mockIssueData })
        .mockResolvedValueOnce({ data: mockComments })
        .mockResolvedValueOnce({ data: mockEvents })
        .mockResolvedValueOnce({ data: mockMilestone });

      const result = await service.getIssue("owner", "repo", 1, {
        includeMetadata: true,
      });

      expect(mockAxios.get).toHaveBeenCalledWith("/repos/owner/repo/issues/1");
      expect(result.title).toBe("Test Issue");
      expect(result.state).toBe(IssueState.Open);
      expect(result.comments).toBe(2);
      expect(result.lastModifiedBy?.login).toBe("modifier");
      expect(result.milestone?.title).toBe("v1.0");
      expect(result.validationRules).toEqual([
        {
          field: "title",
          type: "required",
          message: "Issue title is required",
        },
        {
          field: "title",
          type: "maxLength",
          value: 255,
          message: "Issue title cannot exceed 255 characters",
        },
      ]);
    });

    it("should return cached issue when available and valid", async () => {
      const cachedIssue = {
        ...mockIssueData,
        title: "Cached Issue",
        validationRules: [],
        assignees: [],
        lastUpdated: new Date(),
        updateInProgress: false,
        createdAt: new Date(),
        updatedAt: new Date(),
        user: {
          id: 1,
          login: "user",
          avatarUrl: "https://test.com",
          htmlUrl: "https://test.com",
          createdAt: new Date(),
        },
      };

      cacheManager.get.mockResolvedValueOnce(cachedIssue);

      const result = await service.getIssue("owner", "repo", 1);

      expect(cacheManager.get).toHaveBeenCalledWith("issue:owner:repo:1");
      expect(mockAxios.get).not.toHaveBeenCalled();
      expect(result.title).toBe("Cached Issue");
    });

    it("should handle metadata fetch failures gracefully", async () => {
      mockAxios.get
        .mockResolvedValueOnce({ data: mockIssueData })
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
    });
  });

  describe("createIssue", () => {
    it("should create issue successfully", async () => {
      mockAxios.post.mockResolvedValueOnce({
        data: { ...mockIssueData, title: "New Issue" },
      });

      const result = await service.createIssue("owner", "repo", {
        title: "New Issue",
        body: "Issue body",
      });

      expect(mockAxios.post).toHaveBeenCalledWith("/repos/owner/repo/issues", {
        title: "New Issue",
        body: "Issue body",
      });
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
      const updatedIssue = {
        ...mockIssueData,
        title: "Updated Issue",
        state: "closed",
      };

      mockAxios.patch.mockResolvedValueOnce({ data: updatedIssue });

      const result = await service.updateIssue("owner", "repo", 1, {
        title: "Updated Issue",
        state: IssueState.Closed,
      });

      expect(mockAxios.patch).toHaveBeenCalledWith(
        "/repos/owner/repo/issues/1",
        {
          title: "Updated Issue",
          state: IssueState.Closed,
        },
      );
      expect(result.title).toBe("Updated Issue");
      expect(result.state).toBe(IssueState.Closed);
    });

    it("should handle API errors", async () => {
      const error = new AxiosError();
      error.response = {
        data: { message: "Issue not found" },
        status: 404,
        statusText: "Not Found",
        headers: {},
        config: {} as any,
      };

      mockAxios.patch.mockRejectedValueOnce(error);

      await expect(
        service.updateIssue("owner", "repo", 1, { title: "Updated" }),
      ).rejects.toThrow(ApiError);
    });
  });

  describe("updateTitle", () => {
    it("should update title successfully with optimistic updates", async () => {
      const originalIssue = {
        ...mockIssueData,
        title: "Original Title",
      };

      const updatedIssue = {
        ...mockIssueData,
        title: "New Title",
        updated_at: new Date().toISOString(),
      };

      mockAxios.get.mockResolvedValueOnce({ data: originalIssue });
      mockAxios.patch.mockResolvedValueOnce({ data: updatedIssue });

      const result = await service.updateTitle(
        "owner",
        "repo",
        1,
        "New Title",
        { optimistic: true },
      );

      expect(cacheManager.set).toHaveBeenCalledWith(
        "issue:owner:repo:1",
        expect.objectContaining({
          title: "New Title",
          updateInProgress: true,
        }),
        300,
      );

      expect(result.title).toBe("New Title");
    });

    it("should handle concurrent updates", async () => {
      const inProgressIssue = {
        ...mockIssueData,
        updateInProgress: true,
        title: "Current Title",
        validationRules: [],
      };

      // Mock both cache and API to return the in-progress issue
      cacheManager.get.mockResolvedValueOnce(inProgressIssue);
      mockAxios.get.mockResolvedValueOnce({
        data: inProgressIssue,
        status: 200,
      });

      try {
        await service.updateTitle("owner", "repo", 1, "New Title");
        fail("Expected error was not thrown");
      } catch (error) {
        if (error instanceof ValidationError) {
          expect(error.message).toBe("Update already in progress");
          expect(error.context).toEqual({
            issueId: 1,
            currentTitle: "Current Title",
          });
        } else {
          throw new Error("Expected ValidationError but got: " + String(error));
        }
      }
    });

    it("should handle rollback on failure", async () => {
      // Create a complete issue object for testing
      const originalIssue = mockIssueData;

      const error = new AxiosError();
      error.response = {
        data: { message: "Update failed" },
        status: 500,
        statusText: "Internal Server Error",
        headers: {},
        config: {} as any,
      };

      // Mock initial state
      const cachedIssue = {
        ...originalIssue,
        title: "Original Title",
        updateInProgress: false,
        updateError: null,
      };

      mockAxios.get.mockResolvedValueOnce({
        data: originalIssue,
        status: 200,
        statusText: "OK",
        headers: {},
        config: {} as any,
      });

      // Mock cache to return the original issue
      cacheManager.get.mockResolvedValueOnce(cachedIssue);

      // Mock the error response
      mockAxios.patch.mockRejectedValueOnce(
        new AxiosError("Update failed", "500", {} as any, {} as any, {
          data: { message: "Update failed" },
          status: 500,
          statusText: "Internal Server Error",
          headers: {},
          config: {} as any,
        }),
      );
      mockAxios.patch.mockRejectedValueOnce(error);

      // Clear previous mock calls
      cacheManager.set.mockClear();

      try {
        await service.updateTitle("owner", "repo", 1, "New Title", {
          optimistic: true,
        });
        throw new Error("Expected error was not thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(ApiError);

        // Get the last call to cacheManager.set
        // Get the last cache update
        const lastCall =
          cacheManager.set.mock.calls[cacheManager.set.mock.calls.length - 1];
        const cachedIssue = lastCall[1] as Issue;

        // Verify the cache key and TTL
        expect(lastCall[0]).toBe("issue:owner:repo:1");
        expect(lastCall[2]).toBe(300);

        // Verify the rollback state
        expect(cachedIssue.title).toBe("Original Title");
        expect(cachedIssue.updateInProgress).toBe(false);
        expect(cachedIssue.updateError).toBe("Update failed");

        // Verify other properties remain unchanged
        expect(cachedIssue.id).toBe(originalIssue.id);
        expect(cachedIssue.number).toBe(originalIssue.number);
        expect(cachedIssue.state).toBe(originalIssue.state);
        expect(cacheManager.set).toHaveBeenCalledTimes(2);
        expect(cacheManager.set).toHaveBeenCalledWith(
          "issue:owner:repo:1",
          expect.objectContaining({
            title: "Original Title",
            updateInProgress: false,
            updateError: "Update failed",
          }),
          300,
        );
      }
    });
  });

  describe("Type Guards", () => {
    describe("isIssue", () => {
      it("should validate valid issue object structure", () => {
        expect(isIssue(mockIssueData)).toBe(true);
      });

      it("should reject invalid issue objects", () => {
        expect(isIssue(null)).toBe(false);
        expect(isIssue({})).toBe(false);
        expect(isIssue({ id: 1 })).toBe(false);
        // Test invalid state
        expect(
          isIssue({
            ...mockIssueData,
            state: "invalid" as IssueState, // Force invalid state
            validationRules: [],
          }),
        ).toBe(false);

        // Test missing required fields
        expect(
          isIssue({
            ...mockIssueData,
            user: null,
            validationRules: [],
          }),
        ).toBe(false);

        // Test invalid date fields
        expect(
          isIssue({
            ...mockIssueData,
            createdAt: "invalid-date",
            validationRules: [],
          }),
        ).toBe(false);
      });
    });

    describe("isMilestone", () => {
      it("should validate valid milestone object structure", () => {
        expect(isMilestone(mockMilestone)).toBe(true);
      });

      it("should reject invalid milestone objects", () => {
        expect(isMilestone(null)).toBe(false);
        expect(isMilestone({})).toBe(false);
        expect(isMilestone({ id: 1 })).toBe(false);
        // Test invalid state
        expect(
          isMilestone({
            ...mockMilestone,
            state: "invalid" as "open" | "closed",
          }),
        ).toBe(false);

        // Test missing required fields
        expect(
          isMilestone({
            id: 1,
            number: 1,
            description: "test",
            state: "open",
          }),
        ).toBe(false);

        // Test invalid date fields
        expect(
          isMilestone({
            ...mockMilestone,
            createdAt: "invalid-date",
          }),
        ).toBe(false);
      });
    });
  });
});
