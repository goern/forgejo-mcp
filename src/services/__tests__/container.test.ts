import "reflect-metadata";
import { Container } from "inversify";
import { createContainer, createTestContainer } from "../../container.js";
import { TYPES } from "../../types/di.js";
import { CodebergService } from "../codeberg.service.js";
import { ErrorHandler } from "../error-handler.service.js";
import { Logger } from "../logger.service.js";
import type { ICodebergService, IErrorHandler, ILogger } from "../types.js";

describe("DI Container", () => {
  describe("createContainer", () => {
    let container: Container;
    const config = {
      baseUrl: "https://codeberg.org/api/v1",
      token: "test-token",
      timeout: 5000,
      maxRetries: 3,
    };

    beforeEach(() => {
      container = createContainer(config);
    });

    it("should create a container with all required bindings", () => {
      expect(container.isBound(TYPES.CodebergService)).toBe(true);
      expect(container.isBound(TYPES.ErrorHandler)).toBe(true);
      expect(container.isBound(TYPES.Logger)).toBe(true);
      expect(container.isBound(TYPES.Config)).toBe(true);
      expect(container.isBound(TYPES.ServiceName)).toBe(true);
    });

    it("should resolve CodebergService as singleton", () => {
      const service1 = container.get<ICodebergService>(TYPES.CodebergService);
      const service2 = container.get<ICodebergService>(TYPES.CodebergService);
      expect(service1).toBeInstanceOf(CodebergService);
      expect(service1).toBe(service2);
    });

    it("should resolve ErrorHandler as singleton", () => {
      const handler1 = container.get<IErrorHandler>(TYPES.ErrorHandler);
      const handler2 = container.get<IErrorHandler>(TYPES.ErrorHandler);
      expect(handler1).toBeInstanceOf(ErrorHandler);
      expect(handler1).toBe(handler2);
    });

    it("should resolve Logger as singleton", () => {
      const logger1 = container.get<ILogger>(TYPES.Logger);
      const logger2 = container.get<ILogger>(TYPES.Logger);
      expect(logger1).toBeInstanceOf(Logger);
      expect(logger1).toBe(logger2);
    });

    it("should inject config correctly", () => {
      const injectedConfig = container.get(TYPES.Config);
      expect(injectedConfig).toEqual(config);
    });

    it("should inject service name correctly", () => {
      const serviceName = container.get(TYPES.ServiceName);
      expect(serviceName).toBe("CodebergService");
    });
  });

  describe("createTestContainer", () => {
    let container: Container;

    beforeEach(() => {
      container = createTestContainer();
    });

    it("should create a container with test configuration", () => {
      const config = container.get(TYPES.Config);
      expect(config).toHaveProperty(
        "baseUrl",
        "https://test.codeberg.org/api/v1",
      );
      expect(config).toHaveProperty("token", "test-token");
      expect(config).toHaveProperty("timeout", 1000);
      expect(config).toHaveProperty("maxRetries", 1);
    });

    it("should resolve all required services", () => {
      expect(() => container.get(TYPES.CodebergService)).not.toThrow();
      expect(() => container.get(TYPES.ErrorHandler)).not.toThrow();
      expect(() => container.get(TYPES.Logger)).not.toThrow();
    });
  });
});
