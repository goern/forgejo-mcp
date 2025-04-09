// Type guards for tool arguments
export const isValidRepoArgs = (
  args: any,
): args is { owner: string; name?: string } =>
  typeof args === "object" &&
  args !== null &&
  typeof args.owner === "string" &&
  (args.name === undefined || typeof args.name === "string");

export const isValidIssueArgs = (
  args: any,
): args is { owner: string; repo: string; number?: number; state?: string } =>
  typeof args === "object" &&
  args !== null &&
  typeof args.owner === "string" &&
  typeof args.repo === "string" &&
  (args.number === undefined || typeof args.number === "number") &&
  (args.state === undefined || typeof args.state === "string");

export const isValidCreateIssueArgs = (
  args: any,
): args is { owner: string; repo: string; title: string; body: string } =>
  typeof args === "object" &&
  args !== null &&
  typeof args.owner === "string" &&
  typeof args.repo === "string" &&
  typeof args.title === "string" &&
  typeof args.body === "string";

export const isValidUserArgs = (args: any): args is { username: string } =>
  typeof args === "object" &&
  args !== null &&
  typeof args.username === "string";
