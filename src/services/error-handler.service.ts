import axios, { AxiosError } from "axios";
import { injectable } from "inversify";
import {
  ApiError,
  ForgejoError,
  NetworkError,
  type IErrorHandler,
  type ToolResponse,
} from "./types.js";

/**
 * Handles error processing, mapping, and retry logic for Codeberg API operations
 */
@injectable()
export class ErrorHandler implements IErrorHandler {
  /**
   * Maps API errors to appropriate error types and formats
   */
  handleApiError(error: unknown): never {
    // Handle Axios errors
    if (axios.isAxiosError(error)) {
      const axiosError = error as AxiosError;

      // Network or timeout errors
      if (!axiosError.response) {
        throw new NetworkError(axiosError.message || "Network error occurred", {
          code: axiosError.code,
          cause: axiosError.cause,
        });
      }

      // API errors with response
      const status = axiosError.response.status;
      const data = axiosError.response.data as any;

      throw new ApiError(data?.message || `API error: ${status}`, status, {
        url: axiosError.config?.url,
        method: axiosError.config?.method,
        data: axiosError.response.data,
      });
    }

    // Handle network errors without Axios wrapper
    if (error instanceof Error) {
      if (error.name === "NetworkError") {
        throw new NetworkError(error.message, { originalError: error });
      }

      // Re-throw ForgejoErrors as-is
      if (error instanceof ForgejoError) {
        throw error;
      }
    }

    // Unknown errors
    throw new ForgejoError("An unexpected error occurred", "UNKNOWN_ERROR", {
      error,
    });
  }

  /**
   * Formats errors for tool responses
   */
  handleToolError(error: unknown): ToolResponse {
    let message: string;
    let details: Record<string, unknown> = {};

    const formatError = (err: unknown): Record<string, unknown> => {
      if (err instanceof ForgejoError) {
        const result: Record<string, unknown> = {
          message: err.message,
          code: err.code,
          context: err.context,
        };
        if (err.cause) {
          result.cause = formatError(err.cause);
        }
        return result;
      } else if (err instanceof Error) {
        const result: Record<string, unknown> = {
          message: err.message,
          name: err.name,
        };
        if (err.stack) {
          result.stack = err.stack;
        }
        if (err.cause) {
          result.cause = formatError(err.cause);
        }
        return result;
      }
      return { error: err };
    };

    const replacer = (key: string, value: unknown): unknown => {
      if (key === "self" && typeof value === "object" && value !== null) {
        return "[Circular Reference]";
      }
      return value;
    };

    if (error instanceof ForgejoError) {
      message = error.message;
      details = formatError(error);
    } else if (error instanceof Error) {
      message = error.message;
      details = formatError(error);
    } else {
      message = "An unexpected error occurred";
      details = { error };
    }

    return {
      content: [
        {
          type: "text",
          text: JSON.stringify(
            {
              error: message,
              details,
            },
            replacer,
            2,
          ),
        },
      ],
    };
  }

  /**
   * Determines if an error should trigger a retry attempt
   */
  shouldRetry(error: unknown): boolean {
    // Don't retry validation errors
    if (error instanceof ForgejoError && error.code === "VALIDATION_ERROR") {
      return false;
    }

    // Retry network errors
    if (error instanceof NetworkError) {
      return true;
    }

    // Retry certain API errors
    if (error instanceof ApiError) {
      const status = error.statusCode;

      // Retry server errors and rate limits
      return (
        status >= 500 || // Server errors
        status === 429 || // Rate limit
        status === 408 // Request timeout
      );
    }

    return false;
  }

  /**
   * Calculates delay for retry attempts using exponential backoff
   */
  getRetryDelay(attempt: number): number {
    // Base delay of 1 second
    const baseDelay = 1000;

    // Exponential backoff with jitter
    const exponentialDelay = baseDelay * Math.pow(2, attempt - 1);
    const jitter = Math.random() * 200; // Random delay 0-200ms

    // Maximum delay of 10 seconds
    return Math.min(exponentialDelay + jitter, 10000);
  }
}
