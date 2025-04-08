import {
  ApiError,
  InvalidRepositoryDataError,
  InvalidUserDataError,
} from "../types.js";
import type {
  Repository,
  User,
  Issue,
  IssueState,
  Milestone,
} from "../types.js";

export class CodebergMappers {
  static mapRepository(data: any): Repository {
    if (!data) {
      throw new InvalidRepositoryDataError("Invalid repository data", 400);
    }

    if (!data.owner) {
      throw new InvalidRepositoryDataError(
        "Repository owner data is required",
        400,
      );
    }

    // Create a minimal user object from available owner data
    const owner: User = {
      id: data.owner.id,
      login: data.owner.login,
      fullName: data.owner.full_name || data.owner.login,
      email: data.owner.email || "",
      avatarUrl: data.owner.avatar_url || "",
      htmlUrl: data.owner.html_url || "",
      createdAt: data.owner.created_at
        ? new Date(data.owner.created_at)
        : new Date(),
    };

    return {
      id: data.id,
      name: data.name,
      fullName: data.full_name,
      description: data.description,
      htmlUrl: data.html_url,
      cloneUrl: data.clone_url,
      createdAt: new Date(data.created_at),
      updatedAt: new Date(data.updated_at),
      owner,
    };
  }

  static mapUser(data: any): User {
    if (!data) {
      throw new InvalidUserDataError("Invalid user data", 400);
    }
    return {
      id: data.id,
      login: data.login,
      fullName: data.full_name,
      email: data.email,
      avatarUrl: data.avatar_url,
      htmlUrl: data.html_url,
      createdAt: new Date(data.created_at || Date.now()),
    };
  }

  static mapIssue(data: any): Issue {
    return {
      // Core fields
      id: data.id,
      number: data.number,
      title: data.title,
      body: data.body,
      state: data.state as IssueState,
      htmlUrl: data.html_url,
      createdAt: new Date(data.created_at),
      updatedAt: new Date(data.updated_at),
      user: CodebergMappers.mapUser(data.user),
      labels: (data.labels || []).map((label: any) => ({
        id: label.id,
        name: label.name,
        color: label.color,
        description: label.description,
      })),

      // Enhanced metadata
      lastModifiedBy: data.lastModifiedBy,
      assignees: (data.assignees || [])
        .filter(Boolean)
        .map((u: any) => CodebergMappers.mapUser(u)),
      milestone: data.milestone
        ? CodebergMappers.mapMilestone(data.milestone)
        : undefined,
      comments: data.comments || 0,
      locked: !!data.locked,

      // Update tracking
      lastUpdated: new Date(data.updated_at),
      updateInProgress: false,
      updateError: undefined,

      // Validation
      validationRules: [],
    };
  }

  static mapMilestone(data: any): Milestone {
    return {
      id: data.id,
      number: data.number,
      title: data.title,
      description: data.description,
      dueDate: data.due_on ? new Date(data.due_on) : undefined,
      state: data.state,
      createdAt: new Date(data.created_at),
      updatedAt: new Date(data.updated_at),
    };
  }
}
