import { describe, expect, it, jest, beforeEach } from "@jest/globals";
import axios, { AxiosError, AxiosInstance } from "axios";
import { BaseForgejoService } from "../base.service.js";
import { ErrorHandler } from "../error-handler.service.js";
import { Logger } from "../logger.service.js";
import {
  ApiError,
  ForgejoConfig,
  ICacheManager,
  NetworkError,
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

class TestBaseService extends BaseForgejoService {
  public async testMakeRequest<T>(
    operation: string,
    request: () => Promise<T>,
    context: Record<string, unknown> = {},
  ): Promise<T> {
    return this.makeRequest(operation, request, context);
  }

  public testValidateRepoParams(owner: string, name?: string): void {
    return this.validateRepoParams(owner, name);
  }
}

describe("BaseForgejoService", () => {
  let service: TestBaseService;
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

    service = new TestBaseService(config, errorHandler, logger, cacheManager);
  });

  describe("validateRepoParams", () => {
    it("should validate owner parameter", () => {
      expect(() => service.testValidateRepoParams("")).toThrow(ValidationError);
      expect(() => service.testValidateRepoParams("  ")).toThrow(
        ValidationError,
      );
      expect(() => service.testValidateRepoParams("owner")).not.toThrow();
    });

    it("should validate repository name when provided", () => {
      expect(() => service.testValidateRepoParams("owner", "")).toThrow(
        ValidationError,
      );
      expect(() => service.testValidateRepoParams("owner", "  ")).toThrow(
        ValidationError,
      );
      expect(() =>
        service.testValidateRepoParams("owner", "repo"),
      ).not.toThrow();
    });
  });

  describe("makeRequest", () => {
    it("should make successful request", async () => {
      const mockData = { success: true };
      mockAxios.get.mockResolvedValueOnce({ data: mockData });

      const result = await service.testMakeRequest("test", () =>
        mockAxios.get("/test").then((res) => res.data),
      );

      expect(result).toEqual(mockData);
    });

    it("should handle API errors", async () => {
      const error = new AxiosError();
      error.response = {
        data: { message: "Not found" },
        status: 404,
        statusText: "Not Found",
        headers: {},
        config: {} as any,
      };

      mockAxios.get.mockRejectedValueOnce(error);

      await expect(
        service.testMakeRequest("test", () => mockAxios.get("/test")),
      ).rejects.toThrow(ApiError);
    });

    it("should handle network errors", async () => {
      const error = new AxiosError();
      error.message = "Network Error";
      mockAxios.get.mockRejectedValueOnce(error);

      await expect(
        service.testMakeRequest("test", () => mockAxios.get("/test")),
      ).rejects.toThrow(NetworkError);
    });

    it("should retry failed requests", async () => {
      const error = new AxiosError();
      error.message = "Network Error";
      const mockData = { success: true };

      // Create an API error instead of network error for retry test
      const apiError = new AxiosError();
      apiError.response = {
        data: { message: "Server Error" },
        status: 500,
        statusText: "Server Error",
        headers: {},
        config: {} as any,
      };

      mockAxios.get
        .mockRejectedValueOnce(apiError)
        .mockRejectedValueOnce(apiError)
        .mockResolvedValueOnce({ data: mockData });

      const result = await service.testMakeRequest("test", () =>
        mockAxios.get("/test").then((res) => res.data),
      );

      expect(mockAxios.get).toHaveBeenCalledTimes(3);
      expect(result).toEqual(mockData);
    });

    it("should respect maxRetries configuration", async () => {
      const apiError = new AxiosError();
      apiError.response = {
        data: { message: "Server Error" },
        status: 500,
        statusText: "Server Error",
        headers: {},
        config: {} as any,
      };

      mockAxios.get.mockRejectedValue(apiError);

      await expect(
        service.testMakeRequest("test", () => mockAxios.get("/test")),
      ).rejects.toThrow("Server Error");

      expect(mockAxios.get).toHaveBeenCalledTimes(3); // maxRetries = 3
    });
  });
});
