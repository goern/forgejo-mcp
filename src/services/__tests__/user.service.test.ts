import { describe, expect, it, jest, beforeEach } from "@jest/globals";
import axios, { AxiosError, AxiosInstance } from "axios";
import { UserService } from "../user.service.js";
import { ErrorHandler } from "../error-handler.service.js";
import { Logger } from "../logger.service.js";
import {
  ApiError,
  ForgejoConfig,
  ICacheManager,
  ValidationError,
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

describe("UserService", () => {
  let service: UserService;
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

    service = new UserService(config, errorHandler, logger, cacheManager);
  });

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

      mockAxios.get.mockResolvedValueOnce({ data: mockUser });

      const result = await service.getCurrentUser();

      expect(mockAxios.get).toHaveBeenCalledWith("/user");
      expect(result.login).toBe("user");
      expect(result.fullName).toBe("Test User");
      expect(result.email).toBe("user@example.com");
    });

    it("should handle invalid response data", async () => {
      mockAxios.get.mockResolvedValueOnce({ data: null });

      await expect(service.getCurrentUser()).rejects.toThrow(ApiError);
    });

    it("should handle API errors", async () => {
      const error = new AxiosError();
      error.response = {
        data: { message: "Unauthorized" },
        status: 401,
        statusText: "Unauthorized",
        headers: {},
        config: {} as any,
      };

      mockAxios.get.mockRejectedValueOnce(error);

      await expect(service.getCurrentUser()).rejects.toThrow(ApiError);
    });
  });

  describe("getUser", () => {
    it("should get user successfully", async () => {
      const mockUser = {
        id: 1,
        login: "testuser",
        full_name: "Test User",
        email: "testuser@example.com",
        avatar_url: "https://codeberg.org/avatar/1",
        html_url: "https://codeberg.org/testuser",
        created_at: "2025-01-01T00:00:00Z",
      };

      mockAxios.get.mockResolvedValueOnce({ data: mockUser });

      const result = await service.getUser("testuser");

      expect(mockAxios.get).toHaveBeenCalledWith("/users/testuser");
      expect(result.login).toBe("testuser");
      expect(result.fullName).toBe("Test User");
      expect(result.email).toBe("testuser@example.com");
    });

    it("should throw ValidationError for empty username", async () => {
      await expect(service.getUser("")).rejects.toThrow(ValidationError);
      await expect(service.getUser("  ")).rejects.toThrow(ValidationError);
    });

    it("should handle user not found", async () => {
      const error = new AxiosError();
      error.response = {
        data: { message: "User not found" },
        status: 404,
        statusText: "Not Found",
        headers: {},
        config: {} as any,
      };

      mockAxios.get.mockRejectedValueOnce(error);

      await expect(service.getUser("nonexistent")).rejects.toThrow(ApiError);
    });

    it("should handle invalid response data", async () => {
      mockAxios.get.mockResolvedValueOnce({ data: null });

      await expect(service.getUser("testuser")).rejects.toThrow(ApiError);
    });
  });
});
