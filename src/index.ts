#!/usr/bin/env node
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import * as http from "http";
import {
  CallToolRequestSchema,
  ErrorCode,
  ListResourcesRequestSchema,
  ListResourceTemplatesRequestSchema,
  ListToolsRequestSchema,
  McpError,
  ReadResourceRequestSchema,
  ServerResult,
} from "@modelcontextprotocol/sdk/types.js";
import * as dotenv from "dotenv";
import { existsSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";
import yargs from "yargs";
import { hideBin } from "yargs/helpers";
import { createContainer } from "./container.js";
import type {
  ICodebergService,
  IErrorHandler,
  ILogger,
} from "./services/types.js";
import { IssueState } from "./services/types.js";
import { TYPES } from "./container.js";

// Parse command line arguments
const argv = yargs(hideBin(process.argv))
  .options({
    sse: {
      type: "boolean",
      description: "Run in SSE mode instead of stdio mode",
      default: false,
    },
    port: {
      type: "number",
      description: "Port to use for SSE mode",
      default: 3000,
    },
    host: {
      type: "string",
      description: "Host to bind to for SSE mode",
      default: "localhost",
    },
  })
  .help()
  .alias("help", "h")
  .version(false)
  .parseSync();

// Load environment variables from .env file if it exists
const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const envPath = join(dirname(__dirname), ".env");

if (existsSync(envPath)) {
  console.error(`Loading environment variables from ${envPath}`);
  dotenv.config({ path: envPath });
}

// Get API token from environment variable
const API_TOKEN = process.env.CODEBERG_API_TOKEN;
if (!API_TOKEN) {
  throw new Error(
    "CODEBERG_API_TOKEN environment variable is required. " +
      "You can set it in a .env file in the project root or in the MCP settings.",
  );
}

// Type guards for tool arguments
const isValidRepoArgs = (args: any): args is { owner: string; name?: string } =>
  typeof args === "object" &&
  args !== null &&
  typeof args.owner === "string" &&
  (args.name === undefined || typeof args.name === "string");

const isValidIssueArgs = (
  args: any,
): args is { owner: string; repo: string; number?: number; state?: string } =>
  typeof args === "object" &&
  args !== null &&
  typeof args.owner === "string" &&
  typeof args.repo === "string" &&
  (args.number === undefined || typeof args.number === "number") &&
  (args.state === undefined || typeof args.state === "string");

const isValidCreateIssueArgs = (
  args: any,
): args is { owner: string; repo: string; title: string; body: string } =>
  typeof args === "object" &&
  args !== null &&
  typeof args.owner === "string" &&
  typeof args.repo === "string" &&
  typeof args.title === "string" &&
  typeof args.body === "string";

const isValidUserArgs = (args: any): args is { username: string } =>
  typeof args === "object" &&
  args !== null &&
  typeof args.username === "string";

class CodebergServer {
  private server: Server;
  private codebergService: ICodebergService;
  private errorHandler: IErrorHandler;
  private logger: ILogger;

  constructor() {
    // Initialize DI container
    const container = createContainer({
      baseUrl: "https://codeberg.org/api/v1",
      token: API_TOKEN,
      timeout: 10000,
      maxRetries: 3,
    });

    // Get service instances
    this.codebergService = container.get<ICodebergService>(
      TYPES.CodebergService,
    );
    this.errorHandler = container.get<IErrorHandler>(TYPES.ErrorHandler);
    this.logger = container.get<ILogger>(TYPES.Logger);

    this.server = new Server(
      {
        name: "codeberg-server",
        version: "0.1.0",
      },
      {
        capabilities: {
          resources: {},
          tools: {},
        },
      },
    );

    this.setupResourceHandlers();
    this.setupToolHandlers();

    // Error handling
    this.server.onerror = (error) => this.logger.error("[MCP Error]", error);
    process.on("SIGINT", async () => {
      await this.server.close();
      process.exit(0);
    });
  }

  private setupResourceHandlers(): void {
    // List available resources
    this.server.setRequestHandler(
      ListResourcesRequestSchema,
      async (_, extra): Promise<ServerResult> => {
        return {
          resources: [
            {
              uri: `codeberg://user/profile`,
              name: `Current user profile`,
              mimeType: "application/json",
              description: "Profile information for the authenticated user",
            },
          ],
        };
      },
    );

    // Resource templates
    this.server.setRequestHandler(
      ListResourceTemplatesRequestSchema,
      async (_, extra): Promise<ServerResult> => ({
        resourceTemplates: [
          {
            uriTemplate: "codeberg://repos/{owner}/{repo}",
            name: "Repository information",
            mimeType: "application/json",
            description: "Details about a specific repository",
          },
          {
            uriTemplate: "codeberg://repos/{owner}/{repo}/issues",
            name: "Repository issues",
            mimeType: "application/json",
            description: "List of issues for a repository",
          },
          {
            uriTemplate: "codeberg://users/{username}",
            name: "User information",
            mimeType: "application/json",
            description: "Details about a specific user",
          },
        ],
      }),
    );

    // Handle resource requests
    this.server.setRequestHandler(
      ReadResourceRequestSchema,
      async (request, extra): Promise<ServerResult> => {
        const uri = request.params.uri;

        try {
          // Current user profile
          if (uri === "codeberg://user/profile") {
            const user = await this.codebergService.getCurrentUser();
            return {
              contents: [
                {
                  uri,
                  mimeType: "application/json",
                  text: JSON.stringify(user, null, 2),
                },
              ],
            };
          }

          // Repository information
          const repoMatch = uri.match(/^codeberg:\/\/repos\/([^/]+)\/([^/]+)$/);
          if (repoMatch) {
            const [, owner, repo] = repoMatch;
            const repository = await this.codebergService.getRepository(
              owner,
              repo,
            );
            return {
              contents: [
                {
                  uri,
                  mimeType: "application/json",
                  text: JSON.stringify(repository, null, 2),
                },
              ],
            };
          }

          // Repository issues
          const issuesMatch = uri.match(
            /^codeberg:\/\/repos\/([^/]+)\/([^/]+)\/issues$/,
          );
          if (issuesMatch) {
            const [, owner, repo] = issuesMatch;
            const issues = await this.codebergService.listIssues(owner, repo);
            return {
              contents: [
                {
                  uri,
                  mimeType: "application/json",
                  text: JSON.stringify(issues, null, 2),
                },
              ],
            };
          }

          // User information
          const userMatch = uri.match(/^codeberg:\/\/users\/([^/]+)$/);
          if (userMatch) {
            const [, username] = userMatch;
            const user = await this.codebergService.getUser(username);
            return {
              contents: [
                {
                  uri,
                  mimeType: "application/json",
                  text: JSON.stringify(user, null, 2),
                },
              ],
            };
          }

          throw new McpError(
            ErrorCode.InvalidRequest,
            `Invalid URI format: ${uri}`,
          );
        } catch (error) {
          const result = this.errorHandler.handleToolError(error);
          return {
            contents: [
              {
                uri,
                mimeType: "application/json",
                text: JSON.stringify(result, null, 2),
              },
            ],
          };
        }
      },
    );
  }

  private setupToolHandlers(): void {
    this.server.setRequestHandler(
      ListToolsRequestSchema,
      async (_, extra): Promise<ServerResult> => ({
        tools: [
          {
            name: "list_repositories",
            description: "List repositories for a user or organization",
            inputSchema: {
              type: "object",
              properties: {
                owner: {
                  type: "string",
                  description: "Username or organization name",
                },
              },
              required: ["owner"],
            },
          },
          {
            name: "get_repository",
            description: "Get details about a specific repository",
            inputSchema: {
              type: "object",
              properties: {
                owner: {
                  type: "string",
                  description: "Repository owner",
                },
                name: {
                  type: "string",
                  description: "Repository name",
                },
              },
              required: ["owner", "name"],
            },
          },
          {
            name: "list_issues",
            description: "List issues for a repository",
            inputSchema: {
              type: "object",
              properties: {
                owner: {
                  type: "string",
                  description: "Repository owner",
                },
                repo: {
                  type: "string",
                  description: "Repository name",
                },
                state: {
                  type: "string",
                  description: "Issue state (open, closed, all)",
                  enum: ["open", "closed", "all"],
                },
              },
              required: ["owner", "repo"],
            },
          },
          {
            name: "get_issue",
            description: "Get details about a specific issue",
            inputSchema: {
              type: "object",
              properties: {
                owner: {
                  type: "string",
                  description: "Repository owner",
                },
                repo: {
                  type: "string",
                  description: "Repository name",
                },
                number: {
                  type: "number",
                  description: "Issue number",
                },
              },
              required: ["owner", "repo", "number"],
            },
          },
          {
            name: "create_issue",
            description: "Create a new issue in a repository",
            inputSchema: {
              type: "object",
              properties: {
                owner: {
                  type: "string",
                  description: "Repository owner",
                },
                repo: {
                  type: "string",
                  description: "Repository name",
                },
                title: {
                  type: "string",
                  description: "Issue title",
                },
                body: {
                  type: "string",
                  description: "Issue body",
                },
              },
              required: ["owner", "repo", "title", "body"],
            },
          },
          {
            name: "get_user",
            description: "Get details about a user",
            inputSchema: {
              type: "object",
              properties: {
                username: {
                  type: "string",
                  description: "Username",
                },
              },
              required: ["username"],
            },
          },
        ],
      }),
    );

    this.server.setRequestHandler(
      CallToolRequestSchema,
      async (request, extra): Promise<ServerResult> => {
        try {
          switch (request.params.name) {
            case "list_repositories": {
              if (!isValidRepoArgs(request.params.arguments)) {
                throw new McpError(
                  ErrorCode.InvalidParams,
                  "Invalid repository arguments",
                );
              }

              const { owner } = request.params.arguments;
              const repositories =
                await this.codebergService.listRepositories(owner);
              return {
                content: [
                  {
                    type: "text",
                    text: JSON.stringify(repositories, null, 2),
                  },
                ],
              };
            }

            case "get_repository": {
              if (
                !isValidRepoArgs(request.params.arguments) ||
                !request.params.arguments.name
              ) {
                throw new McpError(
                  ErrorCode.InvalidParams,
                  "Invalid repository arguments",
                );
              }

              const { owner, name } = request.params.arguments;
              const repository = await this.codebergService.getRepository(
                owner,
                name,
              );
              return {
                content: [
                  {
                    type: "text",
                    text: JSON.stringify(repository, null, 2),
                  },
                ],
              };
            }

            case "list_issues": {
              if (!isValidIssueArgs(request.params.arguments)) {
                throw new McpError(
                  ErrorCode.InvalidParams,
                  "Invalid issue arguments",
                );
              }

              const { owner, repo, state } = request.params.arguments;
              const issues = await this.codebergService.listIssues(
                owner,
                repo,
                {
                  state: state as IssueState,
                },
              );
              return {
                content: [
                  {
                    type: "text",
                    text: JSON.stringify(issues, null, 2),
                  },
                ],
              };
            }

            case "get_issue": {
              if (
                !isValidIssueArgs(request.params.arguments) ||
                !request.params.arguments.number
              ) {
                throw new McpError(
                  ErrorCode.InvalidParams,
                  "Invalid issue arguments",
                );
              }

              const { owner, repo, number } = request.params.arguments;
              const issue = await this.codebergService.getIssue(
                owner,
                repo,
                number,
              );
              return {
                content: [
                  {
                    type: "text",
                    text: JSON.stringify(issue, null, 2),
                  },
                ],
              };
            }

            case "create_issue": {
              if (!isValidCreateIssueArgs(request.params.arguments)) {
                throw new McpError(
                  ErrorCode.InvalidParams,
                  "Invalid issue arguments",
                );
              }

              const { owner, repo, title, body } = request.params.arguments;
              const issue = await this.codebergService.createIssue(
                owner,
                repo,
                { title, body },
              );
              return {
                content: [
                  {
                    type: "text",
                    text: JSON.stringify(issue, null, 2),
                  },
                ],
              };
            }

            case "get_user": {
              if (!isValidUserArgs(request.params.arguments)) {
                throw new McpError(
                  ErrorCode.InvalidParams,
                  "Invalid user arguments",
                );
              }

              const { username } = request.params.arguments;
              const user = await this.codebergService.getUser(username);
              return {
                content: [
                  {
                    type: "text",
                    text: JSON.stringify(user, null, 2),
                  },
                ],
              };
            }

            default:
              throw new McpError(
                ErrorCode.InvalidRequest,
                `Unknown tool: ${request.params.name}`,
              );
          }
        } catch (error) {
          const result = this.errorHandler.handleToolError(error);
          return {
            content: [
              {
                type: "text",
                text: JSON.stringify(result, null, 2),
              },
            ],
          };
        }
      },
    );
  }

  public async run(): Promise<void> {
    if (argv.sse) {
      // Run in SSE mode
      const httpServer = http.createServer((req, res) => {
        res.setHeader("Access-Control-Allow-Origin", "*");
        res.setHeader("Access-Control-Allow-Methods", "POST, OPTIONS");
        res.setHeader("Access-Control-Allow-Headers", "Content-Type");

        if (req.method === "OPTIONS") {
          res.writeHead(200);
          res.end();
          return;
        }
      });

      httpServer.listen(argv.port, argv.host, () => {
        this.logger.info(
          `Server running in SSE mode at http://${argv.host}:${argv.port}`,
        );
      });

      process.on("SIGINT", () => {
        httpServer.close();
        process.exit(0);
      });
    } else {
      // Run in stdio mode
      const transport = new StdioServerTransport();
      await this.server.connect(transport);
    }
  }
}

// Start the server
const server = new CodebergServer();
server.run().catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
