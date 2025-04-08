import { injectable } from "inversify";
import axios from "axios";
import { BaseForgejoService } from "./base.service.js";
import { ForgejoMappers } from "./utils/mappers.js";
import {
  ApiError,
  ValidationError,
  type Issue,
  type CreateIssueData,
  type UpdateIssueData,
  type ListIssueOptions,
  IssueState,
  isIssue,
} from "./types.js";

@injectable()
export class IssueService extends BaseForgejoService {
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
        let issuesResponse;
        try {
          issuesResponse = await this.axiosInstance.get(
            `/repos/${owner}/${repo}/issues`,
            { params: options },
          );
        } catch (error) {
          throw this.errorHandler.handleApiError(error);
        }
        this.logger.debug(`request successful`, {
          ...issuesResponse,
        });

        /*
                        const response = await this.axiosInstance.get(
                            `/repos/${owner}/${repo}/issues`,
                            { params: options },
                        );
                        console.log(response)
                        if (!response?.data) {
                            throw new ApiError("Invalid response from server", 500);
                        }*/

        return Array.isArray(issuesResponse.data)
          ? issuesResponse.data.map(ForgejoMappers.mapIssue)
          : [];
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
        let metadata = {};
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
                ? ForgejoMappers.mapUser(eventsResponse.data[0].actor)
                : undefined,
              milestone: milestoneResponse.data
                ? ForgejoMappers.mapMilestone(milestoneResponse.data)
                : undefined,
            };
          } catch (error) {
            this.logger.error(
              "Failed to fetch issue metadata",
              error instanceof Error ? error : new Error(String(error)),
            );
          }
        }

        const issue = ForgejoMappers.mapIssue({
          ...issueResponse.data,
          ...metadata,
        });

        // Add validation rules
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
          const ttl = issue.state === IssueState.Closed ? 3600 : 300;
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
        return ForgejoMappers.mapIssue(response.data);
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

          return ForgejoMappers.mapIssue(response.data);
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

      // Check if update is already in progress
      if (originalIssue.updateInProgress) {
        throw new ValidationError("Update already in progress", {
          issueId: number,
          currentTitle: originalIssue.title,
        });
      }

      // Apply optimistic update if enabled
      if (options.optimistic && this.cacheManager) {
        const optimisticIssue = {
          ...originalIssue,
          title: newTitle,
          updateInProgress: true,
          lastUpdated: new Date(),
        };
        await this.cacheManager.set(cacheKey, optimisticIssue, 300);
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
      // If it's a validation error, rethrow it
      if (error instanceof ValidationError) {
        throw error;
      }

      // Log the error
      this.logger.warn(`updateIssue request failed`, {
        error,
        owner,
        repo,
        number,
      });

      // Handle error and rollback if needed
      if (options.optimistic && originalIssue && this.cacheManager) {
        try {
          const errorMessage = "Update failed";

          // Rollback to original state
          await this.cacheManager.set(
            cacheKey,
            {
              ...originalIssue,
              title: originalIssue.title,
              updateError: errorMessage,
              updateInProgress: false,
            },
            300,
          );

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
}
