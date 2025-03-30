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
} from "@modelcontextprotocol/sdk/types.js";
import axios from "axios";
import * as dotenv from "dotenv";
import { existsSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";
import yargs from "yargs";
import { hideBin } from "yargs/helpers";

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

// Codeberg API base URL
const CODEBERG_API_BASE_URL = "https://codeberg.org/api/v1";

// Get API token from environment variable
const API_TOKEN = process.env.CODEBERG_API_TOKEN;
if (!API_TOKEN) {
  throw new Error(
    "CODEBERG_API_TOKEN environment variable is required. " +
      "You can set it in a .env file in the project root or in the MCP settings.",
  );
}

// Type definitions for Codeberg API responses
interface CodebergRepository {
  id: number;
  name: string;
  full_name: string;
  description: string;
  html_url: string;
  clone_url: string;
  created_at: string;
  updated_at: string;
  owner: {
    id: number;
    login: string;
    avatar_url: string;
    html_url: string;
  };
}

interface CodebergIssue {
  id: number;
  number: number;
  title: string;
  body: string;
  state: string;
  html_url: string;
  created_at: string;
  updated_at: string;
  user: {
    id: number;
    login: string;
    avatar_url: string;
    html_url: string;
  };
}

interface CodebergUser {
  id: number;
  login: string;
  full_name: string;
  email: string;
  avatar_url: string;
  html_url: string;
  created_at: string;
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
  private axiosInstance;

  constructor() {
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

    this.axiosInstance = axios.create({
      baseURL: CODEBERG_API_BASE_URL,
      headers: {
        Authorization: `token ${API_TOKEN}`,
        "Content-Type": "application/json",
        Accept: "application/json",
      },
    });

    this.setupResourceHandlers();
    this.setupToolHandlers();

    // Error handling
    this.server.onerror = (error) => console.error("[MCP Error]", error);
    process.on("SIGINT", async () => {
      await this.server.close();
      process.exit(0);
    });
  }

  private setupResourceHandlers() {
    // List available resources
    this.server.setRequestHandler(ListResourcesRequestSchema, async () => ({
      resources: [
        {
          uri: `codeberg://user/profile`,
          name: `Current user profile`,
          mimeType: "application/json",
          description: "Profile information for the authenticated user",
        },
      ],
    }));

    // Resource templates
    this.server.setRequestHandler(
      ListResourceTemplatesRequestSchema,
      async () => ({
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
      async (request) => {
        const uri = request.params.uri;

        // Current user profile
        if (uri === "codeberg://user/profile") {
          try {
            const response = await this.axiosInstance.get("/user");
            return {
              contents: [
                {
                  uri,
                  mimeType: "application/json",
                  text: JSON.stringify(response.data, null, 2),
                },
              ],
            };
          } catch (error) {
            this.handleAxiosError(error);
          }
        }

        // Repository information
        const repoMatch = uri.match(/^codeberg:\/\/repos\/([^/]+)\/([^/]+)$/);
        if (repoMatch) {
          const [, owner, repo] = repoMatch;
          try {
            const response = await this.axiosInstance.get(
              `/repos/${owner}/${repo}`,
            );
            return {
              contents: [
                {
                  uri,
                  mimeType: "application/json",
                  text: JSON.stringify(response.data, null, 2),
                },
              ],
            };
          } catch (error) {
            this.handleAxiosError(error);
          }
        }

        // Repository issues
        const issuesMatch = uri.match(
          /^codeberg:\/\/repos\/([^/]+)\/([^/]+)\/issues$/,
        );
        if (issuesMatch) {
          const [, owner, repo] = issuesMatch;
          try {
            const response = await this.axiosInstance.get(
              `/repos/${owner}/${repo}/issues`,
            );
            return {
              contents: [
                {
                  uri,
                  mimeType: "application/json",
                  text: JSON.stringify(response.data, null, 2),
                },
              ],
            };
          } catch (error) {
            this.handleAxiosError(error);
          }
        }

        // User information
        const userMatch = uri.match(/^codeberg:\/\/users\/([^/]+)$/);
        if (userMatch) {
          const [, username] = userMatch;
          try {
            const response = await this.axiosInstance.get(`/users/${username}`);
            return {
              contents: [
                {
                  uri,
                  mimeType: "application/json",
                  text: JSON.stringify(response.data, null, 2),
                },
              ],
            };
          } catch (error) {
            this.handleAxiosError(error);
          }
        }

        throw new McpError(
          ErrorCode.InvalidRequest,
          `Invalid URI format: ${uri}`,
        );
      },
    );
  }

  private setupToolHandlers() {
    this.server.setRequestHandler(ListToolsRequestSchema, async () => ({
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
    }));

    this.server.setRequestHandler(CallToolRequestSchema, async (request) => {
      switch (request.params.name) {
        case "list_repositories": {
          if (!isValidRepoArgs(request.params.arguments)) {
            throw new McpError(
              ErrorCode.InvalidParams,
              "Invalid repository arguments",
            );
          }

          const { owner } = request.params.arguments;

          try {
            const response = await this.axiosInstance.get(
              `/users/${owner}/repos`,
            );
            return {
              content: [
                {
                  type: "text",
                  text: JSON.stringify(response.data, null, 2),
                },
              ],
            };
          } catch (error) {
            return this.handleToolError(error);
          }
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

          try {
            const response = await this.axiosInstance.get(
              `/repos/${owner}/${name}`,
            );
            return {
              content: [
                {
                  type: "text",
                  text: JSON.stringify(response.data, null, 2),
                },
              ],
            };
          } catch (error) {
            return this.handleToolError(error);
          }
        }

        case "list_issues": {
          if (!isValidIssueArgs(request.params.arguments)) {
            throw new McpError(
              ErrorCode.InvalidParams,
              "Invalid issue arguments",
            );
          }

          const { owner, repo, state = "open" } = request.params.arguments;

          try {
            const response = await this.axiosInstance.get(
              `/repos/${owner}/${repo}/issues`,
              {
                params: { state },
              },
            );
            return {
              content: [
                {
                  type: "text",
                  text: JSON.stringify(response.data, null, 2),
                },
              ],
            };
          } catch (error) {
            return this.handleToolError(error);
          }
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

          try {
            const response = await this.axiosInstance.get(
              `/repos/${owner}/${repo}/issues/${number}`,
            );
            return {
              content: [
                {
                  type: "text",
                  text: JSON.stringify(response.data, null, 2),
                },
              ],
            };
          } catch (error) {
            return this.handleToolError(error);
          }
        }

        case "create_issue": {
          if (!isValidCreateIssueArgs(request.params.arguments)) {
            throw new McpError(
              ErrorCode.InvalidParams,
              "Invalid create issue arguments",
            );
          }

          const { owner, repo, title, body } = request.params.arguments;

          try {
            const response = await this.axiosInstance.post(
              `/repos/${owner}/${repo}/issues`,
              {
                title,
                body,
              },
            );
            return {
              content: [
                {
                  type: "text",
                  text: JSON.stringify(response.data, null, 2),
                },
              ],
            };
          } catch (error) {
            return this.handleToolError(error);
          }
        }

        case "get_user": {
          if (!isValidUserArgs(request.params.arguments)) {
            throw new McpError(
              ErrorCode.InvalidParams,
              "Invalid user arguments",
            );
          }

          const { username } = request.params.arguments;

          try {
            const response = await this.axiosInstance.get(`/users/${username}`);
            return {
              content: [
                {
                  type: "text",
                  text: JSON.stringify(response.data, null, 2),
                },
              ],
            };
          } catch (error) {
            return this.handleToolError(error);
          }
        }

        default:
          throw new McpError(
            ErrorCode.MethodNotFound,
            `Unknown tool: ${request.params.name}`,
          );
      }
    });
  }

  private handleAxiosError(error: any): never {
    if (axios.isAxiosError(error)) {
      throw new McpError(
        ErrorCode.InternalError,
        `Codeberg API error: ${error.response?.data.message ?? error.message}`,
      );
    }
    throw error;
  }

  private handleToolError(error: any) {
    if (axios.isAxiosError(error)) {
      return {
        content: [
          {
            type: "text",
            text: `Codeberg API error: ${
              error.response?.data.message ?? error.message
            }`,
          },
        ],
        isError: true,
      };
    }
    throw error;
  }

  async run() {
    if (argv.sse) {
      // Create a simple HTTP server
      const httpServer = http.createServer((req, res) => {
        // Serve a simple HTML page with information about the MCP server
        res.writeHead(200, { "Content-Type": "text/html" });
        res.end(`
                    <!DOCTYPE html>
                    <html>
                    <head>
                        <title>Codeberg MCP Server</title>
                        <style>
                            body {
                                font-family: Arial, sans-serif;
                                max-width: 800px;
                                margin: 0 auto;
                                padding: 20px;
                            }
                            pre {
                                background-color: #f5f5f5;
                                padding: 10px;
                                border-radius: 5px;
                                overflow-x: auto;
                            }
                        </style>
                    </head>
                    <body>
                        <h1>Codeberg MCP Server</h1>
                        <p>The MCP server is running in HTTP mode on port ${argv.port}.</p>
                        
                        <h2>Available Tools</h2>
                        <ul>
                            <li><strong>list_repositories</strong>: List repositories for a user or organization</li>
                            <li><strong>get_repository</strong>: Get details about a specific repository</li>
                            <li><strong>list_issues</strong>: List issues for a repository</li>
                            <li><strong>get_issue</strong>: Get details about a specific issue</li>
                            <li><strong>create_issue</strong>: Create a new issue in a repository</li>
                            <li><strong>get_user</strong>: Get details about a user</li>
                        </ul>
                        
                        <h2>Available Resources</h2>
                        <ul>
                            <li><code>codeberg://user/profile</code>: Current user profile</li>
                            <li><code>codeberg://repos/{owner}/{repo}</code>: Repository information</li>
                            <li><code>codeberg://repos/{owner}/{repo}/issues</code>: Repository issues</li>
                            <li><code>codeberg://users/{username}</code>: User information</li>
                        </ul>
                        
                        <h2>Configuration</h2>
                        <p>The server is configured with the following options:</p>
                        <pre>
Host: ${argv.host}
Port: ${argv.port}
Mode: HTTP
                        </pre>
                    </body>
                    </html>
                `);
      });

      // Start the HTTP server
      httpServer.listen(argv.port, argv.host, () => {
        console.error(
          `Codeberg MCP server running in HTTP mode on http://${argv.host}:${argv.port}`,
        );
      });

      // Use Stdio transport for the MCP server
      const transport = new StdioServerTransport();
      await this.server.connect(transport);

      // Handle server shutdown
      process.on("SIGINT", () => {
        httpServer.close();
        process.exit(0);
      });
    } else {
      // Use Stdio transport (default)
      const transport = new StdioServerTransport();
      await this.server.connect(transport);
      console.error("Codeberg MCP server running in stdio mode");
    }
  }
}

const server = new CodebergServer();
server.run().catch(console.error);
