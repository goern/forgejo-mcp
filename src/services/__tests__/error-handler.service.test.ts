import { describe, expect, it, beforeEach } from "@jest/globals";
import { AxiosError } from "axios";
import { ErrorHandler } from "../error-handler.service.js";
import {
  ApiError,
  ForgejoError,
  NetworkError,
  ValidationError,
} from "../types.js";

describe("ErrorHandler", () => {
  let errorHandler: ErrorHandler;

  beforeEach(() => {
    errorHandler = new ErrorHandler();
  });

  describe("handleApiError", () => {
    it("should handle network errors", () => {
      const error = new Error("Network error");
      error.name = "NetworkError";

      expect(() => errorHandler.handleApiError(error)).toThrow(NetworkError);
      expect(() => errorHandler.handleApiError(error)).toThrow("Network error");
    });

    it("should handle API errors with response", () => {
      const error = {
        response: {
          status: 404,
          data: { message: "Not found" },
        },
        config: {
          url: "/test",
          method: "GET",
        },
        isAxiosError: true,
      } as AxiosError;

      expect(() => errorHandler.handleApiError(error)).toThrow(ApiError);
      expect(() => errorHandler.handleApiError(error)).toThrow("Not found");
    });

    it("should pass through ForgejoErrors", () => {
      const error = new ValidationError("Invalid input");

      expect(() => errorHandler.handleApiError(error)).toThrow(ValidationError);
      expect(() => errorHandler.handleApiError(error)).toThrow("Invalid input");
    });

    it("should handle unknown errors", () => {
      const error = { random: "error" };

      expect(() => errorHandler.handleApiError(error)).toThrow(ForgejoError);
      expect(() => errorHandler.handleApiError(error)).toThrow(
        "An unexpected error occurred",
      );
    });
  });

  describe("handleToolError", () => {
    it("should format ForgejoError for tool response", () => {
      const error = new ApiError("API error", 404, { path: "/test" });
      const response = errorHandler.handleToolError(error);

      const parsed = JSON.parse(response.content[0].text) as {
        error: string;
        details: {
          code: string;
          context: { path: string };
        };
      };

      expect(parsed.error).toBe("API error");
      expect(parsed.details.code).toBe("API_ERROR");
      expect(parsed.details.context).toEqual({ path: "/test" });
    });

    it("should format standard Error for tool response", () => {
      const error = new Error("Standard error");
      const response = errorHandler.handleToolError(error);

      const parsed = JSON.parse(response.content[0].text) as {
        error: string;
        details: {
          name: string;
          stack: string;
        };
      };

      expect(parsed.error).toBe("Standard error");
      expect(parsed.details.name).toBe("Error");
      expect(parsed.details.stack).toBeDefined();
    });

    it("should format unknown error for tool response", () => {
      const error = { message: "Unknown error" };
      const response = errorHandler.handleToolError(error);

      const parsed = JSON.parse(response.content[0].text) as {
        error: string;
        details: {
          error: { message: string };
        };
      };

      expect(parsed.error).toBe("An unexpected error occurred");
      expect(parsed.details.error).toEqual({ message: "Unknown error" });
    });
  });

  describe("shouldRetry", () => {
    it("should not retry validation errors", () => {
      const error = new ValidationError("Invalid input");
      expect(errorHandler.shouldRetry(error)).toBe(false);
    });

    it("should retry network errors", () => {
      const error = new NetworkError("Connection failed");
      expect(errorHandler.shouldRetry(error)).toBe(true);
    });

    it("should retry server errors", () => {
      const error = new ApiError("Server error", 500);
      expect(errorHandler.shouldRetry(error)).toBe(true);
    });

    it("should retry rate limit errors", () => {
      const error = new ApiError("Rate limit", 429);
      expect(errorHandler.shouldRetry(error)).toBe(true);
    });

    it("should not retry client errors", () => {
      const error = new ApiError("Not found", 404);
      expect(errorHandler.shouldRetry(error)).toBe(false);
    });
  });

  describe("error chaining and context", () => {
    it("should preserve error chain in nested errors", () => {
      const rootError = new Error("Root error");
      const middleError = new Error("Middle error");
      middleError.cause = rootError;
      const topError = new ApiError("Top error", 500);
      topError.cause = middleError;

      const response = errorHandler.handleToolError(topError);
      const parsed = JSON.parse(response.content[0].text) as {
        error: string;
        details: {
          code: string;
          cause: {
            message: string;
            cause: {
              message: string;
            };
          };
        };
      };

      expect(parsed.error).toBe("Top error");
      expect(parsed.details.cause.message).toBe("Middle error");
      expect(parsed.details.cause.cause.message).toBe("Root error");
    });

    it("should handle circular references in error context", () => {
      const obj: any = { name: "test" };
      obj.self = obj; // Create circular reference

      const error = new ApiError("Circular error", 500, obj);
      const response = errorHandler.handleToolError(error);
      const parsed = JSON.parse(response.content[0].text) as {
        error: string;
        details: {
          context: {
            name: string;
            self: string;
          };
        };
      };

      expect(parsed.error).toBe("Circular error");
      expect(parsed.details.context.name).toBe("test");
      expect(parsed.details.context.self).toBe("[Circular Reference]");
    });

    it("should preserve error context through the chain", () => {
      const context = { requestId: "123", userId: "456" };
      const error = new ApiError("API error", 500, context);
      error.cause = new NetworkError("Network error", { connectionId: "789" });

      const response = errorHandler.handleToolError(error);
      const parsed = JSON.parse(response.content[0].text) as {
        error: string;
        details: {
          context: {
            requestId: string;
            userId: string;
          };
          cause: {
            context: {
              connectionId: string;
            };
          };
        };
      };

      expect(parsed.details.context.requestId).toBe("123");
      expect(parsed.details.context.userId).toBe("456");
      expect(parsed.details.cause.context.connectionId).toBe("789");
    });
  });

  describe("getRetryDelay", () => {
    it("should use exponential backoff", () => {
      const firstDelay = errorHandler.getRetryDelay(1);
      const secondDelay = errorHandler.getRetryDelay(2);
      const thirdDelay = errorHandler.getRetryDelay(3);

      expect(secondDelay).toBeGreaterThan(firstDelay);
      expect(thirdDelay).toBeGreaterThan(secondDelay);
    });

    it("should not exceed maximum delay", () => {
      const delay = errorHandler.getRetryDelay(10); // Large attempt number
      expect(delay).toBeLessThanOrEqual(10000); // Max delay is 10 seconds
    });

    it("should include jitter", () => {
      const delays = new Set();
      for (let i = 0; i < 10; i++) {
        delays.add(errorHandler.getRetryDelay(1));
      }
      // With jitter, we should get different delays even for the same attempt number
      expect(delays.size).toBeGreaterThan(1);
    });
  });
});
