# Architecture Plan: F-001 Codeberg Issue Management

## Existing Codebase Analysis

### Reusable Components

1. Server Infrastructure

   - CodebergServer class provides base server setup
   - Authentication and API token handling
   - Error handling mechanisms
   - Basic HTTP server setup

2. API Integration
   - Axios instance configuration
   - Base API error handling
   - Type definitions for basic Codeberg entities

### Required Refactoring

1. Code Organization

   - Extract API operations into dedicated service classes
   - Implement proper separation of concerns
   - Add proper dependency injection

2. Data Models

   - Enhance CodebergIssue interface for labels
   - Add interfaces for todos and checkboxes
   - Implement proper type safety

3. Technical Debt
   - Add proper caching mechanisms
   - Implement comprehensive error handling
   - Add proper logging

## System Overview

The Codeberg Issue Management feature will be implemented as a set of MCP tools that interact with Codeberg's REST API to manage issues. The system will be built using TypeScript and will follow a modular architecture to ensure maintainability and extensibility.

## Architecture Components

### 1. Core Components

#### CodebergService

- Primary interface for interacting with Codeberg's REST API
- Handles authentication and API token management
- Implements rate limiting and request throttling
- Manages HTTP requests with retry mechanisms

#### IssueManager

- High-level interface for issue operations
- Coordinates between different specialized managers
- Handles common issue operations (fetch, update)
- Manages caching of issue data

#### MarkdownParser

- Parses issue descriptions to extract todo items
- Handles markdown checkbox syntax
- Maintains original formatting during updates
- Provides utilities for todo manipulation

### 2. Specialized Managers

#### LabelManager

- Manages issue label operations
- Caches available repository labels
- Validates label operations

#### TodoManager

- Extracts and manages todo items from issue descriptions
- Handles todo status updates
- Preserves description formatting

### 3. Utility Components

#### CacheManager

- Implements caching strategies for API responses
- Manages cache invalidation
- Optimizes performance for frequently accessed data

#### ErrorHandler

- Provides consistent error handling
- Formats user-friendly error messages
- Implements logging for debugging

## Data Models

### Issue

```typescript
interface Issue {
  id: number;
  title: string;
  description: string;
  labels: Label[];
  // Other Codeberg issue fields
}
```

### Label

```typescript
interface Label {
  id: number;
  name: string;
  color: string;
}
```

### Todo

```typescript
interface Todo {
  index: number;
  text: string;
  isChecked: boolean;
  originalText: string;
  lineNumber: number;
}
```

## API Design

### REST Endpoints Used

1. Issue Management

   - GET /api/v1/repos/{owner}/{repo}/issues/{number}
   - PATCH /api/v1/repos/{owner}/{repo}/issues/{number}

2. Label Management
   - GET /api/v1/repos/{owner}/{repo}/labels
   - POST /api/v1/repos/{owner}/{repo}/issues/{number}/labels
   - DELETE /api/v1/repos/{owner}/{repo}/issues/{number}/labels

## Command Interface

### Issue Commands

```typescript
interface IssueCommands {
  updateTitle(issueId: number, title: string): Promise<void>;
  updateDescription(issueId: number, description: string): Promise<void>;
  getIssue(issueId: number): Promise<Issue>;
}
```

### Label Commands

```typescript
interface LabelCommands {
  addLabels(issueId: number, labels: string[]): Promise<void>;
  removeLabels(issueId: number, labels: string[]): Promise<void>;
  getLabels(issueId: number): Promise<Label[]>;
}
```

### Todo Commands

```typescript
interface TodoCommands {
  listTodos(issueId: number): Promise<Todo[]>;
  toggleTodo(issueId: number, todoIndex: number): Promise<void>;
}
```

## Security Considerations

1. Authentication

   - Use API tokens exclusively
   - Store tokens securely using system keychain
   - Implement token rotation mechanism

2. Data Protection
   - Validate all user inputs
   - Sanitize markdown content
   - Implement proper error handling to prevent data leaks

## Performance Optimizations

1. Caching Strategy

   - Cache repository labels
   - Cache issue content with TTL
   - Implement optimistic updates

2. Request Optimization
   - Batch label operations when possible
   - Implement request throttling
   - Use conditional requests (If-Modified-Since)

## Error Handling

1. Network Errors

   - Implement exponential backoff
   - Provide clear error messages
   - Cache last known good state

2. Validation Errors
   - Validate inputs before API calls
   - Provide specific error messages
   - Handle markdown parsing errors gracefully

## Testing Strategy

1. Unit Tests

   - Test each component in isolation
   - Mock API responses
   - Test error handling

2. Integration Tests

   - Test component interactions
   - Test API integration
   - Test caching behavior

3. E2E Tests
   - Test complete workflows
   - Verify command behavior
   - Test error scenarios

## Monitoring and Logging

1. Performance Monitoring

   - Track API response times
   - Monitor cache hit rates
   - Track error rates

2. Logging
   - Log API interactions
   - Log error details
   - Log performance metrics

## Future Extensibility

The architecture is designed to be extensible for future features:

- Support for additional issue operations
- Integration with other issue tracking systems
- Support for batch operations
- Enhanced caching mechanisms

## Dependencies

1. Required Libraries

   - axios: HTTP client
   - typescript: Programming language
   - markdown-it: Markdown parsing
   - keytar: Secure credential storage

2. Development Dependencies
   - jest: Testing framework
   - ts-jest: TypeScript testing support
   - eslint: Code linting
   - prettier: Code formatting

---

## Enhancement: Typed Errors for Mappers & Unit Tests

### Overview

To improve error granularity and test coverage, we will introduce **specific typed error classes** for the data mappers in `CodebergMappers`. This enables more elegant error handling and clearer debugging, and supports robust unit testing.

### Typed Error Classes

- `InvalidRepositoryDataError` (extends `ApiError`, status code 400)
- `InvalidUserDataError` (extends `ApiError`, status code 400)
- (Optionally extendable with `InvalidIssueDataError`, `InvalidMilestoneDataError` in future)

These errors replace generic `ApiError` throws within mappers, providing precise failure semantics.

### Refactoring Plan

- Refactor `CodebergMappers` methods:
  - Throw `InvalidRepositoryDataError` when repository data is invalid or missing owner.
  - Throw `InvalidUserDataError` when user data is invalid.
- Maintain existing API surface, improving internal error specificity.

### Testing Strategy

- **Unit tests** (using Jest) will be added in `src/services/__tests__/mappers.test.ts`.
- Cover:
  - Successful mappings with valid data.
  - Error throwing on invalid or missing data, asserting correct error class and status code.
  - Edge cases (e.g., missing optional fields, date parsing).
- This ensures elegant, reliable data transformation and error signaling.

### Epics & Tasks

- **Epic:** Typed Error Handling & Mapper Unit Tests
  - **Task 1:** Define new typed error classes extending `ApiError`.
  - **Task 2:** Refactor `CodebergMappers` to use these errors.
  - **Task 3:** Implement comprehensive Jest unit tests.
  - **Task 4:** Update relevant documentation.

### Rationale

This enhancement aligns with our goals of **comprehensive error handling** (see sections 89-94, 202-214) and **robust unit testing** (sections 215-233), improving maintainability and clarity.

---
