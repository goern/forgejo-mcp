import { injectable } from "inversify";
import { BaseCodebergService } from "./base.service.js";
import { CodebergMappers } from "./utils/mappers.js";
import { ValidationError, ApiError, type User } from "./types.js";

@injectable()
export class UserService extends BaseCodebergService {
  /**
   * Gets details about a specific user
   */
  async getUser(username: string): Promise<User> {
    if (!username?.trim()) {
      throw new ValidationError("Username is required");
    }

    return this.makeRequest(
      "getUser",
      async () => {
        const response = await this.axiosInstance.get(`/users/${username}`);
        if (!response?.data) {
          throw new ApiError("Invalid response from server", 500);
        }
        return CodebergMappers.mapUser(response.data);
      },
      { username },
    );
  }

  /**
   * Gets details about the authenticated user
   */
  async getCurrentUser(): Promise<User> {
    return this.makeRequest("getCurrentUser", async () => {
      const response = await this.axiosInstance.get("/user");
      if (!response?.data) {
        throw new ApiError("Invalid response from server", 500);
      }
      return CodebergMappers.mapUser(response.data.data || response.data);
    });
  }
}
