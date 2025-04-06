# Task: T-015_enhance_getissue_command

## Parent Epic

EP-002_issue_management

## Description

Enhance the getIssue command to support the extended CodebergIssue interface and provide additional metadata needed for issue management operations. This is a core MVP feature that will enable proper issue viewing and updates.

## Acceptance Criteria

- [ ] Update getIssue to return enhanced issue metadata
- [ ] Implement caching using EP-001 CacheManager
- [ ] Add error handling for all potential API failures
- [ ] Ensure proper typing with extended CodebergIssue interface
- [ ] Add unit tests for new functionality
- [ ] Document all new parameters and return types

## Technical Details

Implementation requirements:

- Use EP-001 core infrastructure for API calls
- Implement proper error handling using ErrorHandler service
- Cache responses using CacheManager
- Return enhanced issue data structure

Example usage:

```typescript
const issue = await getIssue({
  id: number,
  includeMetadata: boolean,
  forceFresh: boolean,
});
```

Response handling:

- Cache successful responses
- Handle API errors gracefully
- Return properly typed CodebergIssue object
- Include all required metadata for updates

## Dependencies

- T-013 (Extended interface) must be completed
- EP-001 core infrastructure services

## Estimation

2 story points (2-3 days)

## Priority

High (MVP Phase 1)

## Assigned To

Unassigned
