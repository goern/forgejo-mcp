# Task: T-003_extract_api_operations

## Overview

Extract API operations from the monolithic CodebergServer class into a dedicated CodebergService class, implementing the designed service interfaces.

## Acceptance Criteria

- [ ] Create CodebergService class implementing ICodebergService interface
- [ ] Move all API operations from CodebergServer to CodebergService
- [ ] Implement proper error handling for API operations
- [ ] Add request/response type definitions
- [ ] Add input validation for API parameters
- [ ] Implement retry mechanism for failed requests
- [ ] Add proper logging for API operations
- [ ] Write unit tests for CodebergService
- [ ] Update existing integration tests
- [ ] Document all public methods and interfaces

## Technical Details

### Implementation Requirements

1. Service Structure

   ```typescript
   class CodebergService implements ICodebergService {
     constructor(
       private readonly config: CodebergConfig,
       private readonly errorHandler: IErrorHandler,
       private readonly logger: ILogger,
     ) {}

     // API methods
     async getIssue(
       owner: string,
       repo: string,
       number: number,
     ): Promise<Issue>;
     async updateIssue(
       owner: string,
       repo: string,
       number: number,
       data: UpdateIssueData,
     ): Promise<Issue>;
     async getLabels(owner: string, repo: string): Promise<Label[]>;
     // Other API methods
   }
   ```

2. Error Handling

   - Implement request validation
   - Handle network errors
   - Handle API-specific errors
   - Provide detailed error context
   - Use error hierarchies from design

3. Retry Mechanism
   - Implement exponential backoff
   - Handle rate limiting
   - Set maximum retry attempts
   - Log retry attempts

### Required Skills

- TypeScript expertise
- REST API experience
- Unit testing experience
- Error handling patterns
- Logging best practices

### Development Notes

- Follow interface definitions from T-002
- Use axios for HTTP requests
- Implement proper request timeouts
- Consider API rate limits
- Add detailed logging
- Write comprehensive tests

## Dependencies

- T-001: Analysis complete
- T-002: Service layer design complete

## Estimation

3 story points (2-3 days)

## Priority

High (Required for other services)

## References

- T-002 service design document
- Codeberg API documentation
- Current implementation in CodebergServer
- TypeScript best practices
