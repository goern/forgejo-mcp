/**
 * Type definitions for Codeberg API service layer
 */

// Core Models
export interface Repository {
  id: number;
  name: string;
  fullName: string;
  description: string;
  htmlUrl: string;
  cloneUrl: string;
  createdAt: Date;
  updatedAt: Date;
  owner: User;
}

export interface Issue {
  id: number;
  number: number;
  title: string;
  body: string;
  state: IssueState;
  htmlUrl: string;
  createdAt: Date;
  updatedAt: Date;
  user: User;
  labels: Label[];
}

export interface User {
  id: number;
  login: string;
  fullName: string;
  email: string;
  avatarUrl: string;
  htmlUrl: string;
  createdAt: Date;
}

export interface Label {
  id: number;
  name: string;
  color: string;
  description?: string;
}

// DTOs and Options
export interface CreateIssueData {
  title: string;
  body: string;
  labels?: string[];
}

export interface UpdateIssueData {
  title?: string;
  body?: string;
  state?: IssueState;
  labels?: string[];
}

export interface ListIssueOptions {
  state?: IssueState;
  labels?: string[];
  sort?: "created" | "updated" | "comments";
  direction?: "asc" | "desc";
  page?: number;
  perPage?: number;
}

// Enums
export enum IssueState {
  Open = "open",
  Closed = "closed",
  All = "all",
}

// Service Configuration
export interface CodebergConfig {
  baseUrl: string;
  token: string;
  timeout?: number;
  maxRetries?: number;
}

// Error Types
export class CodebergError extends Error {
  constructor(
    message: string,
    public code: string,
    public context?: unknown,
  ) {
    super(message);
    this.name = "CodebergError";
  }
}

export class ApiError extends CodebergError {
  constructor(
    message: string,
    public statusCode: number,
    context?: unknown,
  ) {
    super(message, "API_ERROR", context);
    this.name = "ApiError";
  }
}

export class ValidationError extends CodebergError {
  constructor(message: string, context?: unknown) {
    super(message, "VALIDATION_ERROR", context);
    this.name = "ValidationError";
  }
}

export class NetworkError extends CodebergError {
  constructor(message: string, context?: unknown) {
    super(message, "NETWORK_ERROR", context);
    this.name = "NetworkError";
  }
}

// Service Interfaces
export interface ICodebergService {
  // Repository Operations
  listRepositories(owner: string): Promise<Repository[]>;
  getRepository(owner: string, name: string): Promise<Repository>;

  // Issue Operations
  listIssues(
    owner: string,
    repo: string,
    options?: ListIssueOptions,
  ): Promise<Issue[]>;
  getIssue(owner: string, repo: string, number: number): Promise<Issue>;
  createIssue(
    owner: string,
    repo: string,
    data: CreateIssueData,
  ): Promise<Issue>;
  updateIssue(
    owner: string,
    repo: string,
    number: number,
    data: UpdateIssueData,
  ): Promise<Issue>;

  // User Operations
  getUser(username: string): Promise<User>;
  getCurrentUser(): Promise<User>;
}

export interface IErrorHandler {
  handleApiError(error: unknown): never;
  handleToolError(error: unknown): ToolResponse;
  shouldRetry(error: unknown): boolean;
  getRetryDelay(attempt: number): number;
}

export interface ILogger {
  debug(message: string, context?: Record<string, unknown>): void;
  info(message: string, context?: Record<string, unknown>): void;
  warn(message: string, context?: Record<string, unknown>): void;
  error(
    message: string,
    error?: Error,
    context?: Record<string, unknown>,
  ): void;
}

// Response Types
export interface ToolResponse {
  content: Array<{
    type: string;
    text: string;
  }>;
}
