import {
  describe,
  expect,
  it,
  jest,
  beforeEach,
  afterEach,
} from "@jest/globals";
import type { SpyInstance } from "jest-mock";
import { Logger } from "../logger.service.js";

describe("Logger", () => {
  let logger: Logger;
  let infoSpy: SpyInstance;
  let debugSpy: SpyInstance;
  let warnSpy: SpyInstance;
  let errorSpy: SpyInstance;

  beforeEach(() => {
    logger = new Logger("TestService");
    // Spy on console methods
    infoSpy = jest.spyOn(console, "info").mockImplementation(() => {});
    debugSpy = jest.spyOn(console, "debug").mockImplementation(() => {});
    warnSpy = jest.spyOn(console, "warn").mockImplementation(() => {});
    errorSpy = jest.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  describe("log formatting", () => {
    it("should format basic log message", () => {
      logger.info("Test message");

      const logCall = infoSpy.mock.calls[0][0] as string;
      const parsed = JSON.parse(logCall) as {
        level: string;
        service: string;
        message: string;
        timestamp: string;
      };

      expect(parsed).toEqual(
        expect.objectContaining({
          level: "INFO",
          service: "TestService",
          message: "Test message",
        }),
      );
      expect(parsed.timestamp).toBeDefined();
    });

    it("should include context in log message", () => {
      const context = { userId: 123, action: "test" };
      logger.info("Test with context", context);

      const logCall = infoSpy.mock.calls[0][0] as string;
      const parsed = JSON.parse(logCall) as {
        level: string;
        service: string;
        message: string;
        userId: number;
        action: string;
      };

      expect(parsed).toEqual(
        expect.objectContaining({
          level: "INFO",
          service: "TestService",
          message: "Test with context",
          userId: 123,
          action: "test",
        }),
      );
    });

    it("should format error details", () => {
      const error = new Error("Test error");
      logger.error("Error occurred", error);

      const logCall = errorSpy.mock.calls[0][0] as string;
      const parsed = JSON.parse(logCall) as {
        error: {
          message: string;
          name: string;
          stack: string;
        };
      };

      expect(parsed.error).toBeDefined();
      expect(parsed.error.message).toBe("Test error");
      expect(parsed.error.name).toBe("Error");
      expect(parsed.error.stack).toBeDefined();
    });
  });

  describe("log levels", () => {
    it("should use debug level", () => {
      logger.debug("Debug message");
      expect(debugSpy).toHaveBeenCalled();
    });

    it("should use info level", () => {
      logger.info("Info message");
      expect(infoSpy).toHaveBeenCalled();
    });

    it("should use warn level", () => {
      logger.warn("Warning message");
      expect(warnSpy).toHaveBeenCalled();
    });

    it("should use error level", () => {
      logger.error("Error message");
      expect(errorSpy).toHaveBeenCalled();
    });
  });

  describe("context handling", () => {
    it("should merge multiple contexts", () => {
      const context1 = { userId: 123 };
      const context2 = { requestId: "abc" };

      logger.info("Test message", { ...context1, ...context2 });

      const logCall = infoSpy.mock.calls[0][0] as string;
      const parsed = JSON.parse(logCall) as {
        userId: number;
        requestId: string;
      };

      expect(parsed.userId).toBe(123);
      expect(parsed.requestId).toBe("abc");
    });

    it("should handle undefined context", () => {
      logger.info("Test message");

      const logCall = infoSpy.mock.calls[0][0] as string;
      const parsed = JSON.parse(logCall) as {
        level: string;
        service: string;
        message: string;
      };

      expect(parsed).toEqual(
        expect.objectContaining({
          level: "INFO",
          service: "TestService",
          message: "Test message",
        }),
      );
    });

    it("should handle null values in context", () => {
      logger.info("Test message", { nullValue: null });

      const logCall = infoSpy.mock.calls[0][0] as string;
      const parsed = JSON.parse(logCall) as {
        nullValue: null;
      };

      expect(parsed.nullValue).toBeNull();
    });
  });

  describe("custom error type handling", () => {
    it("should format domain-specific error types", () => {
      class DomainError extends Error {
        constructor(
          message: string,
          public code: string,
          public details: any,
        ) {
          super(message);
          this.name = "DomainError";
        }
      }

      const error = new DomainError("Business rule violation", "BRV001", {
        entity: "user",
        constraint: "uniqueEmail",
      });

      logger.error("Domain error occurred", error);

      const logCall = errorSpy.mock.calls[0][0] as string;
      const parsed = JSON.parse(logCall) as {
        error: {
          name: string;
          message: string;
          code: string;
          details: {
            entity: string;
            constraint: string;
          };
        };
      };

      expect(parsed.error.name).toBe("DomainError");
      expect(parsed.error.code).toBe("BRV001");
      expect(parsed.error.details.entity).toBe("user");
      expect(parsed.error.details.constraint).toBe("uniqueEmail");
    });

    it("should handle error inheritance chain", () => {
      class BaseError extends Error {
        constructor(
          message: string,
          public baseInfo: string,
        ) {
          super(message);
          this.name = "BaseError";
        }
      }

      class ExtendedError extends BaseError {
        constructor(
          message: string,
          baseInfo: string,
          public extraInfo: string,
        ) {
          super(message, baseInfo);
          this.name = "ExtendedError";
        }
      }

      const error = new ExtendedError(
        "Complex error",
        "base-data",
        "extra-data",
      );
      logger.error("Inheritance error occurred", error);

      const logCall = errorSpy.mock.calls[0][0] as string;
      const parsed = JSON.parse(logCall) as {
        error: {
          name: string;
          message: string;
          baseInfo: string;
          extraInfo: string;
        };
      };

      expect(parsed.error.name).toBe("ExtendedError");
      expect(parsed.error.baseInfo).toBe("base-data");
      expect(parsed.error.extraInfo).toBe("extra-data");
    });
  });

  describe("complex context objects", () => {
    it("should handle deeply nested context objects", () => {
      const complexContext = {
        user: {
          profile: {
            address: {
              city: "Test City",
              country: {
                code: "TC",
                name: "Test Country",
              },
            },
          },
          preferences: {
            notifications: {
              email: true,
              push: {
                enabled: true,
                frequency: "daily",
              },
            },
          },
        },
      };

      logger.info("Complex context test", complexContext);

      const logCall = infoSpy.mock.calls[0][0] as string;
      const parsed = JSON.parse(logCall) as {
        user: {
          profile: {
            address: {
              city: string;
              country: {
                code: string;
                name: string;
              };
            };
          };
          preferences: {
            notifications: {
              email: boolean;
              push: {
                enabled: boolean;
                frequency: string;
              };
            };
          };
        };
      };

      expect(parsed.user.profile.address.city).toBe("Test City");
      expect(parsed.user.profile.address.country.code).toBe("TC");
      expect(parsed.user.preferences.notifications.push.frequency).toBe(
        "daily",
      );
    });

    it("should handle arrays and nested collections", () => {
      const contextWithCollections = {
        items: [
          { id: 1, tags: ["a", "b"] },
          { id: 2, tags: ["c", "d"] },
        ],
        metadata: {
          categories: [
            { name: "cat1", subcategories: ["sub1", "sub2"] },
            { name: "cat2", subcategories: ["sub3", "sub4"] },
          ],
        },
      };

      logger.info("Collection context test", contextWithCollections);

      const logCall = infoSpy.mock.calls[0][0] as string;
      const parsed = JSON.parse(logCall) as {
        items: Array<{
          id: number;
          tags: string[];
        }>;
        metadata: {
          categories: Array<{
            name: string;
            subcategories: string[];
          }>;
        };
      };

      expect(parsed.items[0].tags).toEqual(["a", "b"]);
      expect(parsed.metadata.categories[1].subcategories).toEqual([
        "sub3",
        "sub4",
      ]);
    });
  });
});
