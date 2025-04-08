# Team Lead Active Assignments

## EP-001_core_infrastructure (F-001)

### Status

Task breakdown completed and tasks ready for implementation. All tasks have detailed documentation and are properly sequenced.

### Tasks Created

- T-001: Analyze current CodebergServer implementation (2 points)
- T-002: Design service layer architecture (3 points)
- T-003: Extract API operations into CodebergService (3 points)
- T-004: Implement dependency injection container (2 points)
- T-005: Create CacheManager for API responses (2 points)
- T-006: Enhance error handling and logging (2 points)
- T-007: Refactor request handling for modularity (2 points)
- T-008: Update unit tests for refactored services (2 points)
- T-009: Update integration tests (2 points)
- T-010: Update technical documentation (2 points)

### Total Story Points

22 points

### Dependencies

None (foundational epic)

### Implementation Order

1. T-001 -> T-002 (Analysis and Design)
2. T-003 -> T-004 (Core Implementation)
3. T-005 -> T-006 -> T-007 (Infrastructure)
4. T-008 -> T-009 (Testing)
5. T-010 (Documentation)

### Notes

- All tasks have detailed documentation in task directories
- Tasks can be partially parallelized within their groups
- Critical path: T-001 -> T-002 -> T-003 -> T-004

## EP-002_issue_management (F-001)

### Status

Task breakdown in progress, MVP Phase 1 tasks created

### Tasks Created (MVP Phase 1)

- T-011: Analyze existing issue management code (1 point)
- T-013: Extend CodebergIssue interface (1 point)
- T-015: Enhance getIssue command (2 points)
- T-016: Implement updateTitle command (2 points)
- T-019: Update unit tests (2 points)

### Total Story Points (MVP Phase 1)

8 points

### Dependencies

EP-001_core_infrastructure must be completed first

### Implementation Order

1. T-011 (Analysis)
2. T-013 (Interface)
3. T-015 (GetIssue)
4. T-016 (UpdateTitle)
5. T-019 (Tests)

### Notes

- MVP Phase 1 focuses on core functionality
- Additional tasks will be broken down after MVP validation
- Tasks have detailed documentation in task directories
