# Bug Report: TypeScript Build Errors After Adding Cache Manager

## Description

After implementing the enhanced getIssue command with caching support (T-015), the TypeScript build is failing with multiple type-related errors.

## Current Behavior

Running `npm run build` produces 21 TypeScript errors across 4 files:

1. Jest type declaration conflicts:

   - Multiple declarations of jest, describe, expect, it, beforeEach, and afterEach
   - Custom jest.d.ts file conflicts with @types/jest

2. TYPES export issue:

   - Module "./container.js" declares TYPES locally but doesn't export it

3. Mock cache manager type issues:
   - Type mismatches in mock implementations for ICacheManager methods

## Expected Behavior

The TypeScript build should complete successfully without any type errors.

## Steps to Reproduce

1. Check out commit fcc8779
2. Run `npm run build`

## Technical Details

### Key Files Affected

1. src/types/jest.d.ts
2. src/container.ts
3. src/services/**tests**/codeberg.service.test.ts
4. node_modules/@types/jest/index.d.ts

### Error Categories

1. Type Declaration Conflicts:

   ```typescript
   Cannot redeclare block-scoped variable 'jest'
   Cannot redeclare block-scoped variable 'describe'
   // etc.
   ```

2. Export Issues:

   ```typescript
   Module '"./container.js"' declares 'TYPES' locally, but it is not exported.
   ```

3. Mock Implementation Types:

   ```typescript
   Type 'Mock<UnknownFunction>' is not assignable to type 'MockInstance<...>'
   ```

## Severity

High - Blocking the build process

## Impact

- Build process is failing
- Cannot deploy or release new changes
- Affects development workflow

## Proposed Solutions

1. Remove custom jest type declarations and rely on @types/jest
2. Fix TYPES export in container.ts
3. Update mock cache manager implementation with proper typing

## Related Tasks

- T-015: Enhance getIssue command
