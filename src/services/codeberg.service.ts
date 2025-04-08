import { inject, injectable } from "inversify";
import { TYPES } from "../types/di.js";
import {
  type CodebergConfig,
  type ICacheManager,
  type ICodebergService,
  type IErrorHandler,
  type ILogger,
  type Repository,
  type Issue,
  type User,
  type CreateIssueData,
  type UpdateIssueData,
  type ListIssueOptions,
} from "./types.js";
import { RepositoryService } from "./repository.service.js";
import { IssueService } from "./issue.service.js";
import { UserService } from "./user.service.js";

/**
 * Implementation of the Codeberg API service layer.
 * Orchestrates all API operations through specialized service classes.
 */
@injectable()
export class CodebergService implements ICodebergService {
  private readonly repositoryService: RepositoryService;
  private readonly issueService: IssueService;
  private readonly userService: UserService;

  constructor(
    @inject(TYPES.Config) config: CodebergConfig,
    @inject(TYPES.ErrorHandler) errorHandler: IErrorHandler,
    @inject(TYPES.Logger) logger: ILogger,
    @inject(TYPES.CacheManager) cacheManager: ICacheManager,
  ) {
    // Initialize specialized services
    this.repositoryService = new RepositoryService(
      config,
      errorHandler,
      logger,
      cacheManager,
    );
    this.issueService = new IssueService(
      config,
      errorHandler,
      logger,
      cacheManager,
    );
    this.userService = new UserService(
      config,
      errorHandler,
      logger,
      cacheManager,
    );
  }

  // Repository operations
  async listRepositories(owner: string): Promise<Repository[]> {
    return this.repositoryService.listRepositories(owner);
  }

  async getRepository(owner: string, name: string): Promise<Repository> {
    return this.repositoryService.getRepository(owner, name);
  }

  // Issue operations
  async listIssues(
    owner: string,
    repo: string,
    options?: ListIssueOptions,
  ): Promise<Issue[]> {
    return this.issueService.listIssues(owner, repo, options);
  }

  async getIssue(
    owner: string,
    repo: string,
    number: number,
    options?: { includeMetadata?: boolean; forceFresh?: boolean },
  ): Promise<Issue> {
    return this.issueService.getIssue(owner, repo, number, options);
  }

  async createIssue(
    owner: string,
    repo: string,
    data: CreateIssueData,
  ): Promise<Issue> {
    return this.issueService.createIssue(owner, repo, data);
  }

  async updateIssue(
    owner: string,
    repo: string,
    number: number,
    data: UpdateIssueData,
  ): Promise<Issue> {
    return this.issueService.updateIssue(owner, repo, number, data);
  }

  async updateTitle(
    owner: string,
    repo: string,
    number: number,
    newTitle: string,
    options?: { optimistic?: boolean },
  ): Promise<Issue> {
    return this.issueService.updateTitle(
      owner,
      repo,
      number,
      newTitle,
      options,
    );
  }

  // User operations
  async getUser(username: string): Promise<User> {
    return this.userService.getUser(username);
  }

  async getCurrentUser(): Promise<User> {
    return this.userService.getCurrentUser();
  }
}
