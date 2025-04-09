// Define symbols for DI bindings
export const TYPES = {
  CacheManager: Symbol.for("CacheManager"),
  ForgejoService: Symbol.for("ForgejoService"),
  IssueService: Symbol.for("IssueService"),
  ErrorHandler: Symbol.for("ErrorHandler"),
  Logger: Symbol.for("Logger"),
  Config: Symbol.for("Config"),
  ServiceName: Symbol.for("ServiceName"),
};
