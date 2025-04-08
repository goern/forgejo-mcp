import "reflect-metadata";
import { Container } from "inversify";
import {
  IForgejoService,
  IErrorHandler,
  ILogger,
  ICacheManager,
} from "./services/types.js";
import { ForgejoService } from "./services/forgejo.service.js";
import { ErrorHandler } from "./services/error-handler.service.js";
import { Logger } from "./services/logger.service.js";
import { MockCacheManager } from "./services/__tests__/mock-cache-manager.js";
import { TYPES } from "./types/di.js";
export { TYPES };

// Create and configure the container
export function createContainer(config: any): Container {
  const container = new Container();

  // Bind services
  container
    .bind<IForgejoService>(TYPES.ForgejoService)
    .to(ForgejoService)
    .inSingletonScope();
  container
    .bind<IErrorHandler>(TYPES.ErrorHandler)
    .to(ErrorHandler)
    .inSingletonScope();
  container.bind<ILogger>(TYPES.Logger).to(Logger).inSingletonScope();
  container.bind<string>(TYPES.ServiceName).toConstantValue("ForgejoService");
  container
    .bind<ICacheManager>(TYPES.CacheManager)
    .to(MockCacheManager)
    .inSingletonScope();

  // Bind configuration
  container.bind(TYPES.Config).toConstantValue(config);

  return container;
}

// Create test container with mocks
export function createTestContainer(): Container {
  const container = new Container();

  // Bind mock services for testing
  container
    .bind<IForgejoService>(TYPES.ForgejoService)
    .to(ForgejoService)
    .inSingletonScope();
  container
    .bind<IErrorHandler>(TYPES.ErrorHandler)
    .to(ErrorHandler)
    .inSingletonScope();
  container.bind<ILogger>(TYPES.Logger).to(Logger).inSingletonScope();
  container.bind<string>(TYPES.ServiceName).toConstantValue("ForgejoService");
  container
    .bind<ICacheManager>(TYPES.CacheManager)
    .to(MockCacheManager)
    .inSingletonScope();

  // Bind test configuration
  container.bind(TYPES.Config).toConstantValue({
    baseUrl: "https://test.codeberg.org/api/v1",
    token: "test-token",
    timeout: 1000,
    maxRetries: 1,
  });

  return container;
}
