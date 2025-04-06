# Service Layer Architecture Design

## 1. Architecture Overview

### Service Layer Structure

```
┌─────────────────────────────────────┐
│           CodebergServer            │
├─────────────────────────────────────┤
│    ┌─────────┐      ┌─────────┐    │
│    │Resource │      │  Tool   │    │
│    │Handlers │      │Handlers │    │
│    └────┬────┘      └────┬────┘    │
├─────────┼───────────────┼──────────┤
│    ┌────┴────────────┬──┴───┐      │
│    │CodebergService  │Logger│      │
│    └─┬──────────┬────┴──────┘      │
│   ┌──┴───┐  ┌───┴────┐            │
│   │Cache │  │ Error  │            │
│   │Manager│  │Handler │            │
│   └──┬───┘  └───┬────┘            │
│   ┌──┴──────────┴───┐             │
│   │    Auth Service │             │
│   └────────────────┘             │
└─────────────────────────────────────┘
```

## 2. Service Interfaces

### IAuthService

```typescript
interface IAuthService {
  // Token Management
  getToken(): Promise<string>;
  refreshToken(): Promise<void>;
  validateToken(token: string): boolean;

  // Headers
  getAuthHeaders(): Record<string, string>;

  // Events
  onTokenExpired(callback: () => void): void;
}
```

### ICodebergService

```typescript
interface ICodebergService {
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

  // User Operations
  getUser(username: string): Promise<User>;
  getCurrentUser(): Promise<User>;
}
```

### ICacheManager

```typescript
interface ICacheManager {
  // Core Operations
  get<T>(key: string): Promise<T | null>;
  set<T>(key: string, value: T, ttl?: number): Promise<void>;
  delete(key: string): Promise<void>;
  clear(): Promise<void>;

  // Advanced Operations
  getOrSet<T>(key: string, factory: () => Promise<T>, ttl?: number): Promise<T>;
  invalidatePattern(pattern: string): Promise<void>;

  // Events
  onEviction(callback: (key: string) => void): void;
}
```

### IErrorHandler

```typescript
interface IErrorHandler {
  // Error Processing
  handleApiError(error: unknown): never;
  handleToolError(error: unknown): ToolResponse;
  handleResourceError(error: unknown): ResourceResponse;

  // Error Creation
  createError(code: ErrorCode, message: string, context?: unknown): McpError;

  // Error Recovery
  shouldRetry(error: unknown): boolean;
  getRetryDelay(attempt: number): number;
}
```

### ILogger

```typescript
interface ILogger {
  // Log Levels
  debug(message: string, context?: Record<string, unknown>): void;
  info(message: string, context?: Record<string, unknown>): void;
  warn(message: string, context?: Record<string, unknown>): void;
  error(
    message: string,
    error?: Error,
    context?: Record<string, unknown>,
  ): void;

  // Context Management
  withContext(context: Record<string, unknown>): ILogger;

  // Request Tracing
  startRequest(requestId: string): void;
  endRequest(requestId: string): void;
}
```

## 3. Data Models

### Core Models

```typescript
interface Repository {
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

interface Issue {
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

interface User {
  id: number;
  login: string;
  fullName: string;
  email: string;
  avatarUrl: string;
  htmlUrl: string;
  createdAt: Date;
}

interface Label {
  id: number;
  name: string;
  color: string;
  description?: string;
}
```

### DTOs

```typescript
interface CreateIssueData {
  title: string;
  body: string;
  labels?: string[];
}

interface ListIssueOptions {
  state?: IssueState;
  labels?: string[];
  sort?: "created" | "updated" | "comments";
  direction?: "asc" | "desc";
  page?: number;
  perPage?: number;
}
```

## 4. Service Interactions

### Request Flow

1. Client makes request through Tool/Resource handler
2. Request is authenticated via AuthService
3. CodebergService processes request:
   - Checks cache via CacheManager
   - Makes API call if needed
   - Handles errors via ErrorHandler
   - Logs operations via Logger
4. Response is cached if applicable
5. Response is returned to client

### Dependencies

```typescript
class CodebergService {
  constructor(
    private auth: IAuthService,
    private cache: ICacheManager,
    private errorHandler: IErrorHandler,
    private logger: ILogger,
  ) {}
}

class CacheManager {
  constructor(private logger: ILogger) {}
}

class ErrorHandler {
  constructor(private logger: ILogger) {}
}
```

## 5. Error Handling Strategy

### Error Hierarchy

```typescript
class CodebergError extends Error {
  constructor(
    message: string,
    public code: ErrorCode,
    public context?: unknown,
  ) {
    super(message);
  }
}

class ApiError extends CodebergError {}
class AuthError extends CodebergError {}
class CacheError extends CodebergError {}
class ValidationError extends CodebergError {}
```

### Error Handling Flow

1. Service catches operational error
2. Error is wrapped in appropriate CodebergError type
3. Error context is captured
4. Error is logged with context
5. Error is propagated or transformed for client

### Retry Strategy

- Implement exponential backoff
- Retry on network errors
- Retry on rate limits
- Maximum 3 retry attempts

## 6. Caching Strategy

### Cache Keys

- Format: `{service}:{operation}:{params}`
- Example: `repo:list:owner=octocat`

### TTL Values

- Repositories: 1 hour
- Issues: 5 minutes
- Users: 1 hour
- Labels: 24 hours

### Invalidation Strategy

- Invalidate on write operations
- Pattern-based invalidation
- LRU eviction policy
- Maximum cache size: 100MB

## 7. Logging Requirements

### Log Levels

- DEBUG: Detailed debugging information
- INFO: General operational information
- WARN: Warning messages for potential issues
- ERROR: Error messages for actual problems

### Log Format

```typescript
interface LogEntry {
  timestamp: string;
  level: LogLevel;
  message: string;
  requestId?: string;
  context?: Record<string, unknown>;
  error?: {
    message: string;
    stack?: string;
    code?: string;
  };
}
```

### Required Context

- Request ID
- Operation name
- User context
- Performance metrics
- Resource usage

## 8. Dependency Injection Container

### Container Setup

```typescript
interface ServiceContainer {
  // Core Services
  authService: IAuthService;
  codebergService: ICodebergService;
  cacheManager: ICacheManager;
  errorHandler: IErrorHandler;
  logger: ILogger;

  // Registration
  register<T>(token: symbol, implementation: new (...args: any[]) => T): void;

  // Resolution
  resolve<T>(token: symbol): T;
}
```

### Service Registration

```typescript
const SERVICE_TOKENS = {
  AUTH: Symbol("AuthService"),
  CODEBERG: Symbol("CodebergService"),
  CACHE: Symbol("CacheManager"),
  ERROR: Symbol("ErrorHandler"),
  LOGGER: Symbol("Logger"),
} as const;
```

### Lifecycle Management

- Singleton services: Logger, ErrorHandler, CacheManager
- Scoped services: CodebergService (per request)
- Transient services: None

## 9. Implementation Plan

1. Create base interfaces and models
2. Implement core services
3. Setup dependency injection
4. Implement caching
5. Add error handling
6. Configure logging
7. Write tests
8. Document APIs

## 10. Migration Strategy

1. Create new services alongside existing code
2. Gradually migrate functionality
3. Run both implementations in parallel
4. Validate new implementation
5. Switch to new implementation
6. Remove old implementation

## 11. Testing Strategy

### Unit Tests

- Test each service in isolation
- Mock dependencies
- Test error conditions
- Verify cache operations

### Integration Tests

- Test service interactions
- Verify error handling
- Test caching behavior
- Validate logging

### Performance Tests

- Measure response times
- Verify cache effectiveness
- Test under load
- Monitor memory usage
