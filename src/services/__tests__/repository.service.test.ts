import { describe, expect, it, jest, beforeEach } from "@jest/globals";
import axios, { AxiosError, AxiosInstance } from "axios";
import { RepositoryService } from "../repository.service.js";
import { ErrorHandler } from "../error-handler.service.js";
import { Logger } from "../logger.service.js";
import {
  ApiError,
  ForgejoConfig,
  ICacheManager,
  ValidationError,
} from "../types.js";
import { mockRepos, mockRepo } from "../__mocks__/mockRepositories.js";

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

describe("RepositoryService", () => {
  let service: RepositoryService;
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

    service = new RepositoryService(config, errorHandler, logger, cacheManager);
  });

  describe("listRepositories", () => {
    it("should list repositories successfully", async () => {
      mockAxios.get.mockResolvedValueOnce({ data: mockRepos });

      const result = await service.listRepositories("owner");

      expect(mockAxios.get).toHaveBeenCalledWith("/users/owner/repos");
      expect(result).toHaveLength(1);
      expect(result[0].name).toBe("repo1");
      expect(result[0].fullName).toBe("owner/repo1");
    });

    it("should throw ValidationError for empty owner", async () => {
      await expect(service.listRepositories("")).rejects.toThrow(
        ValidationError,
      );
    });

    it("should handle API errors", async () => {
      const error = new AxiosError();
      error.response = {
        data: { message: "User not found" },
        status: 404,
        statusText: "Not Found",
        headers: {},
        config: {} as any,
      };

      mockAxios.get.mockRejectedValueOnce(error);

      await expect(service.listRepositories("owner")).rejects.toThrow(ApiError);
    });
  });

  describe("getRepository", () => {
    it("should get repository successfully", async () => {
      mockAxios.get.mockResolvedValueOnce({ data: mockRepo });

      const result = await service.getRepository("owner", "test-repo");

      expect(mockAxios.get).toHaveBeenCalledWith("/repos/owner/test-repo");
      expect(result.name).toBe("test-repo");
      expect(result.fullName).toBe("owner/test-repo");
      expect(result.description).toBe("Test repository");
      expect(result.owner.login).toBe("owner");
    });

    it("should throw ValidationError for empty repository name", async () => {
      await expect(service.getRepository("owner", "")).rejects.toThrow(
        ValidationError,
      );
    });

    it("should handle repository not found", async () => {
      const error = new AxiosError();
      error.response = {
        data: { message: "Repository not found" },
        status: 404,
        statusText: "Not Found",
        headers: {},
        config: {} as any,
      };

      mockAxios.get.mockRejectedValueOnce(error);

      await expect(
        service.getRepository("owner", "non-existent"),
      ).rejects.toThrow(ApiError);
    });
  });
});
