import axios, { AxiosInstance } from "axios";
import { inject, injectable } from "inversify";
import { TYPES } from "../types/di.js";
import {
  ApiError,
  NetworkError,
  ValidationError,
  type ForgejoConfig,
  type ICacheManager,
  type IErrorHandler,
  type ILogger,
} from "./types.js";

@injectable()
export abstract class BaseForgejoService {
  protected readonly axiosInstance: AxiosInstance;
  protected readonly maxRetries: number;

  constructor(
    @inject(TYPES.Config) protected readonly config: ForgejoConfig,
    @inject(TYPES.ErrorHandler) protected readonly errorHandler: IErrorHandler,
    @inject(TYPES.Logger) protected readonly logger: ILogger,
    @inject(TYPES.CacheManager) protected readonly cacheManager: ICacheManager,
  ) {
    this.maxRetries = config.maxRetries ?? 3;
    this.axiosInstance = axios.create({
      baseURL: config.baseUrl,
      timeout: config.timeout ?? 10000,
      headers: {
        Authorization: `token ${config.token}`,
        "Content-Type": "application/json",
        Accept: "application/json",
      },
    });
  }

  protected validateRepoParams(owner: string, name?: string): void {
    if (!owner?.trim()) {
      throw new ValidationError("Repository owner is required");
    }
    if (name !== undefined && !name.trim()) {
      throw new ValidationError(
        "Repository name cannot be empty when provided",
      );
    }
  }

  protected async makeRequest<T>(
    operation: string,
    request: () => Promise<T>,
    context: Record<string, unknown> = {},
  ): Promise<T> {
    let lastError: unknown;

    for (let attempt = 1; attempt <= this.maxRetries; attempt++) {
      try {
        this.logger.debug(`Making ${operation} request`, {
          attempt,
          ...context,
        });
        const result = await request();
        this.logger.debug(`${operation} request successful`, {
          attempt,
          ...context,
        });
        return result;
      } catch (error) {
        lastError = error;

        // Handle errors
        let handledError: Error;
        if (!error || Object.keys(error).length === 0) {
          // Handle empty error case
          handledError = new ApiError("Request failed", 500, {
            data: {},
          });
        } else if (axios.isAxiosError(error)) {
          if (error.message === "Network Error") {
            // For network errors, create a NetworkError
            handledError = new NetworkError(error.message, {
              url: error.config?.url,
              method: error.config?.method,
            });
          } else if (error.response) {
            // Handle API errors with response
            const errorMessage =
              error.response.data?.message ||
              `API error: ${error.response.status}`;
            const errorContext = {
              url: error.config?.url,
              method: error.config?.method,
              data: error.response.data || {},
            };
            handledError = new ApiError(
              errorMessage,
              error.response.status,
              errorContext,
            );
          } else {
            // Handle other axios errors
            handledError = new NetworkError(
              error.message || "Network error occurred",
              {
                url: error.config?.url,
                method: error.config?.method,
              },
            );
          }
        } else {
          // Handle non-axios errors
          handledError =
            error instanceof Error ? error : new Error(String(error));
        }

        this.logger.warn(`${operation} request failed`, {
          attempt,
          error: handledError,
          ...context,
        });

        // For network errors in the first attempt, throw immediately
        if (handledError instanceof NetworkError && attempt === 1) {
          throw handledError;
        }

        // For other errors or subsequent attempts, check if we should retry
        if (
          this.errorHandler.shouldRetry(handledError) &&
          attempt < this.maxRetries
        ) {
          const delay = this.errorHandler.getRetryDelay(attempt);
          await new Promise((resolve) => setTimeout(resolve, delay));
          continue;
        }

        throw handledError;
      }
    }

    throw lastError instanceof Error ? lastError : new Error(String(lastError));
  }
}
