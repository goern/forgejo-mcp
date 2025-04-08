import { inject, injectable } from "inversify";
import { TYPES } from "../types/di.js";
import type { ILogger } from "./types.js";

/**
 * Log entry structure with optional error details
 */
interface LogEntry {
  timestamp: string;
  level: string;
  service: string;
  message: string;
  error?: Record<string, unknown>;
  [key: string]: unknown;
}

/**
 * Handles structured logging with context tracking and error formatting
 */
@injectable()
export class Logger implements ILogger {
  constructor(
    @inject(TYPES.ServiceName)
    private readonly serviceName: string = "ForgejoService",
  ) {}

  /**
   * Formats a log entry with timestamp, level, and context
   */
  private formatLogEntry(
    level: string,
    message: string,
    context?: Record<string, unknown>,
    error?: Error,
  ): string {
    const logEntry: LogEntry = {
      timestamp: new Date().toISOString(),
      level,
      service: this.serviceName,
      message,
      ...context,
    };

    if (error) {
      const errorObj: Record<string, unknown> = {
        message: error.message,
        name: error.name,
      };

      // Include stack trace if available
      if (error.stack) {
        errorObj.stack = error.stack;
      }

      // Capture all enumerable properties from the error
      for (const key of Object.keys(error)) {
        if (key !== "message" && key !== "name" && key !== "stack") {
          errorObj[key] = (error as any)[key];
        }
      }

      // Handle error cause if present
      if ("cause" in error && error.cause) {
        errorObj.cause = error.cause;
      }

      logEntry.error = errorObj;
    }

    return JSON.stringify(logEntry);
  }

  /**
   * Logs debug level information
   */
  debug(message: string, context?: Record<string, unknown>): void {
    console.debug(this.formatLogEntry("DEBUG", message, context));
  }

  /**
   * Logs general information
   */
  info(message: string, context?: Record<string, unknown>): void {
    console.info(this.formatLogEntry("INFO", message, context));
  }

  /**
   * Logs warning messages
   */
  warn(message: string, context?: Record<string, unknown>): void {
    console.warn(this.formatLogEntry("WARN", message, context));
  }

  /**
   * Logs error messages with error details
   */
  error(
    message: string,
    error?: Error,
    context?: Record<string, unknown>,
  ): void {
    console.error(this.formatLogEntry("ERROR", message, context, error));
  }
}
