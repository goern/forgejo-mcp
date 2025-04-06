import "reflect-metadata";
import { Container } from "inversify";
import { ICodebergService, IErrorHandler, ILogger } from "./services/types.js";
import { CodebergService } from "./services/codeberg.service.js";
import { ErrorHandler } from "./services/error-handler.service.js";
import { Logger } from "./services/logger.service.js";
import { TYPES } from "./types/di.js";

// Create and configure the container
export function createContainer(config: any): Container {
  const container = new Container();

  // Bind services
  container
    .bind<ICodebergService>(TYPES.CodebergService)
    .to(CodebergService)
    .inSingletonScope();
  container
    .bind<IErrorHandler>(TYPES.ErrorHandler)
    .to(ErrorHandler)
    .inSingletonScope();
  container.bind<ILogger>(TYPES.Logger).to(Logger).inSingletonScope();
  container.bind<string>(TYPES.ServiceName).toConstantValue("CodebergService");

  // Bind configuration
  container.bind(TYPES.Config).toConstantValue(config);

  return container;
}

// Create test container with mocks
export function createTestContainer(): Container {
  const container = new Container();

  // Bind mock services for testing
  container
    .bind<ICodebergService>(TYPES.CodebergService)
    .to(CodebergService)
    .inSingletonScope();
  container
    .bind<IErrorHandler>(TYPES.ErrorHandler)
    .to(ErrorHandler)
    .inSingletonScope();
  container.bind<ILogger>(TYPES.Logger).to(Logger).inSingletonScope();
  container.bind<string>(TYPES.ServiceName).toConstantValue("CodebergService");

  // Bind test configuration
  container.bind(TYPES.Config).toConstantValue({
    baseUrl: "https://test.codeberg.org/api/v1",
    token: "test-token",
    timeout: 1000,
    maxRetries: 1,
  });

  return container;
}
