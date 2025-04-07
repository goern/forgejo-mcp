import axios, { AxiosInstance } from "axios";
import { inject, injectable } from "inversify";
import { TYPES } from "../types/di.js";
import {
  ApiError,
  NetworkError,
  ValidationError,
  type CodebergConfig,
  type CreateIssueData,
  type ICacheManager,
  type ICodebergService,
  type IErrorHandler,
  type ILogger,
  type Issue,
  IssueState,
  type ListIssueOptions,
  type Repository,
  type UpdateIssueData,
  type User,
  type Milestone,
  isIssue,
} from "./types.js";

/**
 * Implementation of the Codeberg API service layer.
 * Handles all API operations with proper error handling, retry logic, and logging.
 */
@injectable()
export class CodebergService implements ICodebergService {
  private readonly axiosInstance: AxiosInstance;
  private readonly maxRetries: number;

  constructor(
    @inject(TYPES.Config) private readonly config: CodebergConfig,
    @inject(TYPES.ErrorHandler) private readonly errorHandler: IErrorHandler,
    @inject(TYPES.Logger) private readonly logger: ILogger,
    @inject(TYPES.CacheManager) private readonly cacheManager: ICacheManager,
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

    // Bind methods to ensure correct 'this' context
    this.mapRepository = this.mapRepository.bind(this);
    this.mapIssue = this.mapIssue.bind(this);
    this.mapUser = this.mapUser.bind(this);
  }

  /**
   * Validates repository owner and name parameters
   */
  private validateRepoParams(owner: string, name?: string): void {
    if (!owner?.trim()) {
      throw new ValidationError("Repository owner is required");
    }
    if (name !== undefined && !name.trim()) {
      throw new ValidationError(
        "Repository name cannot be empty when provided",
      );
    }
  }

  /**
   * Makes an API request with retry logic
   */
  private async makeRequest<T>(
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

        // Convert error to proper type for logging and retry decision
        let handledError: Error;
        if (axios.isAxiosError(error)) {
          if (error.response) {
            handledError = new ApiError(
              error.response.data?.message ||
                `API error: ${error.response.status}`,
              error.response.status,
              {
                url: error.config?.url,
                method: error.config?.method,
                data: error.response.data,
              },
            );
          } else {
            handledError = new NetworkError(
              error.message || "Network error occurred",
              {
                url: error.config?.url,
                method: error.config?.method,
              },
            );
          }
        } else {
          handledError =
            error instanceof Error ? error : new Error(String(error));
        }

        this.logger.warn(`${operation} request failed`, {
          attempt,
          error: handledError,
          ...context,
        });

        // Check if we should retry based on the handled error
        if (
          this.errorHandler.shouldRetry(handledError) &&
          attempt < this.maxRetries
        ) {
          const delay = this.errorHandler.getRetryDelay(attempt);
          await new Promise((resolve) => setTimeout(resolve, delay));
          continue;
        }

        // If we're not retrying, throw the handled error
        throw handledError;
      }
    }

    // If we've exhausted retries, throw the last error
    throw lastError instanceof Error ? lastError : new Error(String(lastError));
  }

  /**
   * Lists repositories for a user or organization
   */
  async listRepositories(owner: string): Promise<Repository[]> {
    this.validateRepoParams(owner);

    return this.makeRequest(
      "listRepositories",
      async () => {
        const response = await this.axiosInstance.get(`/users/${owner}/repos`);
        return response.data.map(this.mapRepository);
      },
      { owner },
    );
  }

  /**
   * Gets details about a specific repository
   */
  async getRepository(owner: string, name: string): Promise<Repository> {
    this.validateRepoParams(owner, name);

    return this.makeRequest(
      "getRepository",
      async () => {
        const response = await this.axiosInstance.get(
          `/repos/${owner}/${name}`,
        );
        return this.mapRepository(response.data);
      },
      { owner, name },
    );
  }

  /**
   * Lists issues for a repository
   */
  async listIssues(
    owner: string,
    repo: string,
    options: ListIssueOptions = {},
  ): Promise<Issue[]> {
    this.validateRepoParams(owner, repo);

    return this.makeRequest(
      "listIssues",
      async () => {
        const response = await this.axiosInstance.get(
          `/repos/${owner}/${repo}/issues`,
          { params: options },
        );
        return response.data.map(this.mapIssue);
      },
      { owner, repo, options },
    );
  }

  /**
   * Gets details about a specific issue
   */
  async getIssue(
    owner: string,
    repo: string,
    number: number,
    options: {
      includeMetadata?: boolean;
      forceFresh?: boolean;
    } = {},
  ): Promise<Issue> {
    this.validateRepoParams(owner, repo);
    if (number <= 0) {
      throw new ValidationError("Issue number must be positive", { number });
    }

    const cacheKey = `issue:${owner}:${repo}:${number}`;

    // Check cache if not forcing fresh data
    if (!options.forceFresh && this.cacheManager) {
      const cached = await this.cacheManager.get<Issue>(cacheKey);
      if (cached) {
        // Validate cached data
        if (isIssue(cached)) {
          this.logger.debug("Returning cached issue", { cacheKey });
          return cached;
        } else {
          this.logger.warn("Invalid cached issue data", { cacheKey });
          await this.cacheManager.delete(cacheKey);
        }
      }
    }

    return this.makeRequest(
      "getIssue",
      async () => {
        // Fetch issue data with error handling
        let issueResponse;
        try {
          issueResponse = await this.axiosInstance.get(
            `/repos/${owner}/${repo}/issues/${number}`,
          );
        } catch (error) {
          throw this.errorHandler.handleApiError(error);
        }

        // Fetch additional metadata if requested
        let metadata: {
          lastModifiedBy?: User;
          comments?: number;
          assignees?: User[];
          milestone?: Milestone;
          locked?: boolean;
        } = {};

        if (options.includeMetadata) {
          try {
            const [commentsResponse, eventsResponse, milestoneResponse] =
              await Promise.all([
                this.axiosInstance
                  .get(`/repos/${owner}/${repo}/issues/${number}/comments`)
                  .catch((error) => {
                    this.logger.warn("Failed to fetch comments", { error });
                    return { data: [] };
                  }),
                this.axiosInstance
                  .get(`/repos/${owner}/${repo}/issues/${number}/events`)
                  .catch((error) => {
                    this.logger.warn("Failed to fetch events", { error });
                    return { data: [] };
                  }),
                this.axiosInstance
                  .get(`/repos/${owner}/${repo}/issues/${number}/milestone`)
                  .catch((error) => {
                    this.logger.warn("Failed to fetch milestone", { error });
                    return { data: null };
                  }),
              ]);

            metadata = {
              comments: commentsResponse.data.length,
              lastModifiedBy: eventsResponse.data[0]?.actor
                ? this.mapUser(eventsResponse.data[0].actor)
                : undefined,
              milestone: milestoneResponse.data
                ? {
                    id: milestoneResponse.data.id,
                    number: milestoneResponse.data.number,
                    title: milestoneResponse.data.title,
                    description: milestoneResponse.data.description,
                    dueDate: milestoneResponse.data.due_on
                      ? new Date(milestoneResponse.data.due_on)
                      : undefined,
                    state: milestoneResponse.data.state,
                    createdAt: new Date(milestoneResponse.data.created_at),
                    updatedAt: new Date(milestoneResponse.data.updated_at),
                  }
                : undefined,
            };
          } catch (error) {
            this.logger.error(
              "Failed to fetch issue metadata",
              error instanceof Error ? error : new Error(String(error)),
            );
          }
        }

        const issue = this.mapIssue({
          ...issueResponse.data,
          ...metadata,
        });

        // Add validation rules based on issue state
        issue.validationRules = [
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
        ];

        // Cache the result with TTL based on state
        if (this.cacheManager) {
          const ttl = issue.state === IssueState.Closed ? 3600 : 300; // 1 hour for closed, 5 mins for open
          await this.cacheManager.set(cacheKey, issue, ttl);
        }

        return issue;
      },
      { owner, repo, number, options },
    );
  }

  /**
   * Creates a new issue in a repository
   */
  async createIssue(
    owner: string,
    repo: string,
    data: CreateIssueData,
  ): Promise<Issue> {
    this.validateRepoParams(owner, repo);
    if (!data.title?.trim()) {
      throw new ValidationError("Issue title is required");
    }

    return this.makeRequest(
      "createIssue",
      async () => {
        const response = await this.axiosInstance.post(
          `/repos/${owner}/${repo}/issues`,
          data,
        );
        return this.mapIssue(response.data);
      },
      { owner, repo, data },
    );
  }

  /**
   * Updates an existing issue
   */
  async updateIssue(
    owner: string,
    repo: string,
    number: number,
    data: UpdateIssueData,
  ): Promise<Issue> {
    this.validateRepoParams(owner, repo);
    if (number <= 0) {
      throw new ValidationError("Issue number must be positive", { number });
    }

    return this.makeRequest(
      "updateIssue",
      async () => {
        try {
          const response = await this.axiosInstance.patch(
            `/repos/${owner}/${repo}/issues/${number}`,
            data,
          );

          if (!response?.data) {
            throw new ApiError("Invalid response from server", 500);
          }

          return this.mapIssue(response.data);
        } catch (error) {
          if (axios.isAxiosError(error) && error.response) {
            throw new ApiError(
              error.response.data?.message ||
                `API error: ${error.response.status}`,
              error.response.status,
              {
                url: error.config?.url,
                method: error.config?.method,
                data: error.response.data,
              },
            );
          }
          throw error;
        }
      },
      { owner, repo, number, data },
    );
  }

  /**
   * Updates the title of an issue with optimistic update support
   */
  async updateTitle(
    owner: string,
    repo: string,
    number: number,
    newTitle: string,
    options: { optimistic?: boolean } = {},
  ): Promise<Issue> {
    // Validate parameters
    this.validateRepoParams(owner, repo);
    if (number <= 0) {
      throw new ValidationError("Issue number must be positive", { number });
    }
    if (!newTitle?.trim()) {
      throw new ValidationError("New title cannot be empty");
    }
    if (newTitle.length > 255) {
      throw new ValidationError("Title cannot exceed 255 characters", {
        length: newTitle.length,
      });
    }

    const cacheKey = `issue:${owner}:${repo}:${number}`;
    let originalIssue: Issue | undefined;

    try {
      // Get current issue state for optimistic updates and rollback
      originalIssue = await this.getIssue(owner, repo, number);

      // Apply optimistic update if enabled
      if (options.optimistic && this.cacheManager) {
        const optimisticIssue = {
          ...originalIssue,
          title: newTitle,
          updateInProgress: true,
          lastUpdated: new Date(),
        };
        await this.cacheManager.set(cacheKey, optimisticIssue, 300); // 5 minute TTL
      }

      // Make API call to update title
      const updatedIssue = await this.updateIssue(owner, repo, number, {
        title: newTitle,
      });

      // Update cache with new state
      if (this.cacheManager) {
        const ttl = updatedIssue.state === IssueState.Closed ? 3600 : 300;
        await this.cacheManager.set(cacheKey, updatedIssue, ttl);
      }

      return updatedIssue;
    } catch (error) {
      let attempt = 1; // Assuming attempt is initialized somewhere in the method, otherwise define it properly
      let data = {}; // Assuming data is initialized somewhere in the method, otherwise define it properly
      this.logger.warn(`updateIssue request failed`, {
        attempt,
        error,
        owner,
        repo,
        number,
        data,
      });
      this.logger.debug(
        `Original issue state before rollback:`,
        originalIssue as unknown as Record<string, unknown>,
      );
      // Handle error and rollback if needed
      if (options.optimistic && originalIssue && this.cacheManager) {
        try {
          // Set a consistent error message for the rollback
          const errorMessage = "Update failed";

          // Rollback to original state
          await this.cacheManager.set(
            cacheKey,
            {
              ...originalIssue,
              title: originalIssue.title, // Ensure original title is restored
              updateError: errorMessage,
              updateInProgress: false,
            },
            300,
          );

          this.logger.debug(`Issue state after rollback:`, {
            ...originalIssue,
            title: originalIssue.title,
            updateError: errorMessage,
            updateInProgress: false,
          } as unknown as Record<string, unknown>);

          // Re-throw the original error
          throw error;
        } catch (rollbackError: unknown) {
          this.logger.error(
            "Failed to rollback optimistic update",
            rollbackError instanceof Error
              ? rollbackError
              : new Error(String(rollbackError)),
            { cacheKey },
          );
          throw rollbackError;
        }
      }

      // Re-throw the original error with proper type checking
      if (error instanceof ApiError) {
        throw error;
      }
      if (axios.isAxiosError(error) && error.response) {
        throw new ApiError(
          error.response.data?.message || `API error: ${error.response.status}`,
          error.response.status,
          {
            url: error.config?.url,
            method: error.config?.method,
            data: error.response.data,
          },
        );
      }
      throw new ApiError("Update failed", 500);
    }
  }

  /**
   * Gets details about a specific user
   */
  async getUser(username: string): Promise<User> {
    if (!username?.trim()) {
      throw new ValidationError("Username is required");
    }

    return this.makeRequest(
      "getUser",
      async () => {
        const response = await this.axiosInstance.get(`/users/${username}`);
        return this.mapUser(response.data);
      },
      { username },
    );
  }

  /**
   * Gets details about the authenticated user
   */
  async getCurrentUser(): Promise<User> {
    return this.makeRequest("getCurrentUser", async () => {
      const response = await this.axiosInstance.get("/user");
      return this.mapUser(response.data);
    });
  }

  /**
   * Maps raw repository data to Repository type
   */
  private mapRepository(data: any): Repository {
    return {
      id: data.id,
      name: data.name,
      fullName: data.full_name,
      description: data.description,
      htmlUrl: data.html_url,
      cloneUrl: data.clone_url,
      createdAt: new Date(data.created_at),
      updatedAt: new Date(data.updated_at),
      owner: this.mapUser(data.owner),
    };
  }

  /**
   * Maps raw issue data to Issue type
   */
  private mapIssue(data: any): Issue {
    return {
      // Core fields
      id: data.id,
      number: data.number,
      title: data.title,
      body: data.body,
      state: data.state as IssueState,
      htmlUrl: data.html_url,
      createdAt: new Date(data.created_at),
      updatedAt: new Date(data.updated_at),
      user: this.mapUser(data.user),
      labels: (data.labels || []).map((label: any) => ({
        id: label.id,
        name: label.name,
        color: label.color,
        description: label.description,
      })),

      // Enhanced metadata
      lastModifiedBy: data.lastModifiedBy,
      assignees: (data.assignees || []).map(this.mapUser),
      milestone: data.milestone
        ? {
            id: data.milestone.id,
            number: data.milestone.number,
            title: data.milestone.title,
            description: data.milestone.description,
            dueDate: data.milestone.due_on
              ? new Date(data.milestone.due_on)
              : undefined,
            state: data.milestone.state,
            createdAt: new Date(data.milestone.created_at),
            updatedAt: new Date(data.milestone.updated_at),
          }
        : undefined,
      comments: data.comments || 0,
      locked: !!data.locked,

      // Update tracking
      lastUpdated: new Date(data.updated_at),
      updateInProgress: false,
      updateError: undefined,

      // Validation
      validationRules: [],
    };
  }

  /**
   * Maps raw user data to User type
   */
  private mapUser(data: any): User {
    return {
      id: data.id,
      login: data.login,
      fullName: data.full_name,
      email: data.email,
      avatarUrl: data.avatar_url,
      htmlUrl: data.html_url,
      createdAt: new Date(data.created_at),
    };
  }
}
