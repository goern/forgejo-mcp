import type { jest } from "@jest/globals";

declare global {
  const jest: typeof jest;
  const describe: typeof jest.describe;
  const expect: typeof jest.expect;
  const it: typeof jest.it;
  const beforeEach: typeof jest.beforeEach;
  const afterEach: typeof jest.afterEach;
}
