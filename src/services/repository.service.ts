import { injectable } from "inversify";
import { BaseCodebergService } from "./base.service.js";
import { CodebergMappers } from "./utils/mappers.js";
import { ApiError, type Repository } from "./types.js";

@injectable()
export class RepositoryService extends BaseCodebergService {
  /**
   * Lists repositories for a user or organization
   */
  async listRepositories(owner: string): Promise<Repository[]> {
    this.validateRepoParams(owner);

    return this.makeRequest(
      "listRepositories",
      async () => {
        const response = await this.axiosInstance.get(`/users/${owner}/repos`);
        if (!response?.data) {
          throw new ApiError("Invalid response from server", 500);
        }
        if (!Array.isArray(response.data)) {
          throw new ApiError("Invalid response format", 500);
        }
        return response.data.map(CodebergMappers.mapRepository);
      },
      { owner },
    );
  }

  /**
   * Gets details about a specific repository
   */
  async getRepository(owner: string, name: string): Promise<Repository> {
    this.validateRepoParams(owner, name);

    return this.makeRequest(
      "getRepository",
      async () => {
        const response = await this.axiosInstance.get(
          `/repos/${owner}/${name}`,
        );
        return CodebergMappers.mapRepository(response.data);
      },
      { owner, name },
    );
  }
}
