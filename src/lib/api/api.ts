import { mockApi } from "@/lib/api/mock-api";
import type { Application, AppStatus, Workspace } from "@/types/domain";

type ApplicationLifecycle = "active" | "paused" | "archived";

interface ApplicationResponse {
  id: string;
  name: string;
  slug: string;
  description: string;
  accessMode: "workspace" | "restricted";
  createdBy: string;
  lifecycle: ApplicationLifecycle;
  defaultHostname: string;
  createdAt: string;
  updatedAt: string;
}

interface ApplicationListResponse {
  applications: ApplicationResponse[];
}

export interface WorkspaceMemberResponse {
  id: string;
  email: string;
  displayName: string;
  role: "owner" | "editor" | "viewer";
}

export interface ApplicationAccessResponse {
  accessMode: "workspace" | "restricted";
  createdBy: string;
  grants: Array<Pick<WorkspaceMemberResponse, "id" | "email" | "displayName"> & { role: "editor" | "viewer" }>;
}

export interface InvitationResponse {
  id: string;
  email: string;
  role: "editor" | "viewer";
  expiresAt: string;
}

interface ErrorResponse {
  error?: {
    code?: string;
    message?: string;
    requestId?: string;
  };
}

interface MeResponse {
  id: string;
  email: string;
  displayName: string;
  workspace: Workspace;
}

const apiURL = process.env.NEXT_PUBLIC_SPROUT_API_URL ?? "http://127.0.0.1:8080";
const accents = ["mint", "amber", "blue", "rose"] as const;

export class APIError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code: string,
    readonly requestID?: string,
  ) {
    super(message);
    this.name = "APIError";
  }
}

async function request<T>(path: string, token: string | null, options: RequestInit = {}): Promise<T> {
  if (!token) {
    throw new APIError("Sign in to continue.", 401, "authentication_required");
  }

  const headers = new Headers(options.headers);
  headers.set("Accept", "application/json");
  headers.set("Authorization", `Bearer ${token}`);

  const response = await fetch(`${apiURL}${path}`, {
    ...options,
    headers,
  });

  if (!response.ok) {
    let details: ErrorResponse = {};
    try {
      details = (await response.json()) as ErrorResponse;
    } catch {
      // A non-JSON failure still becomes a safe, predictable client error.
    }

    throw new APIError(
      details.error?.message ?? "Sprout could not complete the request.",
      response.status,
      details.error?.code ?? "request_failed",
      details.error?.requestId,
    );
  }

  return (await response.json()) as T;
}

function applicationStatus(lifecycle: ApplicationLifecycle): AppStatus {
  if (lifecycle === "active") return "running";
  return lifecycle;
}

function applicationAccent(value: string): (typeof accents)[number] {
  const total = Array.from(value).reduce((sum, character) => sum + character.charCodeAt(0), 0);
  return accents[total % accents.length];
}

function updatedLabel(value: string): string {
  const updatedAt = new Date(value);
  const elapsedMinutes = Math.max(0, Math.floor((Date.now() - updatedAt.getTime()) / 60_000));

  if (elapsedMinutes < 1) return "Just now";
  if (elapsedMinutes < 60) return `${elapsedMinutes} min ago`;

  const elapsedHours = Math.floor(elapsedMinutes / 60);
  if (elapsedHours < 24) return `${elapsedHours} hr ago`;

  const elapsedDays = Math.floor(elapsedHours / 24);
  return elapsedDays === 1 ? "Yesterday" : `${elapsedDays} days ago`;
}

function mapApplication(application: ApplicationResponse): Application {
  return {
    id: application.id,
    name: application.name,
    slug: application.slug,
    description: application.description,
    accessMode: application.accessMode,
    createdBy: application.createdBy,
    status: applicationStatus(application.lifecycle),
    url: application.defaultHostname,
    updatedAt: updatedLabel(application.updatedAt),
    accent: applicationAccent(application.id),
  };
}

const applicationAPI = {
  async getWorkspaces(token: string | null): Promise<Workspace[]> {
    return (await request<{ workspaces: Workspace[] }>("/api/v1/workspaces", token)).workspaces;
  },
  async getInvitations(slug: string, token: string | null): Promise<InvitationResponse[]> {
    return (await request<{ invitations: InvitationResponse[] }>(`/api/v1/workspaces/${encodeURIComponent(slug)}/invitations`, token)).invitations;
  },
  createInvitation(slug: string, email: string, role: "editor" | "viewer", token: string | null): Promise<InvitationResponse & { token: string }> {
    return request(`/api/v1/workspaces/${encodeURIComponent(slug)}/invitations`, token,
      { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email, role }) });
  },
  revokeInvitation(slug: string, id: string, token: string | null): Promise<{ ok: boolean }> {
    return request(`/api/v1/workspaces/${encodeURIComponent(slug)}/invitations/${encodeURIComponent(id)}`, token, { method: "DELETE" });
  },
  acceptInvitation(invitationToken: string, token: string | null): Promise<Workspace> {
    return request("/api/v1/invitations/accept", token,
      { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ token: invitationToken }) });
  },
  changeMemberRole(slug: string, id: string, role: "editor" | "viewer", token: string | null): Promise<{ ok: boolean }> {
    return request(`/api/v1/workspaces/${encodeURIComponent(slug)}/members/${encodeURIComponent(id)}`, token,
      { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ role }) });
  },
  removeMember(slug: string, id: string, token: string | null): Promise<{ ok: boolean }> {
    return request(`/api/v1/workspaces/${encodeURIComponent(slug)}/members/${encodeURIComponent(id)}`, token, { method: "DELETE" });
  },
  getMe(token: string | null): Promise<MeResponse> {
    return request<MeResponse>("/api/v1/me", token);
  },

  getWorkspace(workspaceSlug: string, token: string | null): Promise<Workspace> {
    return request<Workspace>(`/api/v1/workspaces/${encodeURIComponent(workspaceSlug)}`, token);
  },

  async getApplications(workspaceSlug: string, token: string | null): Promise<Application[]> {
    const response = await request<ApplicationListResponse>(
      `/api/v1/workspaces/${encodeURIComponent(workspaceSlug)}/applications`,
      token,
    );
    return response.applications.map(mapApplication);
  },

  async getApplication(workspaceSlug: string, applicationID: string, token: string | null): Promise<Application> {
    const response = await request<ApplicationResponse>(
      `/api/v1/workspaces/${encodeURIComponent(workspaceSlug)}/applications/${encodeURIComponent(applicationID)}`,
      token,
    );
    return mapApplication(response);
  },

  async getWorkspaceMembers(workspaceSlug: string, token: string | null): Promise<WorkspaceMemberResponse[]> {
    const response = await request<{ members: WorkspaceMemberResponse[] }>(
      `/api/v1/workspaces/${encodeURIComponent(workspaceSlug)}/members`, token,
    );
    return response.members;
  },

  getApplicationAccess(workspaceSlug: string, applicationID: string, token: string | null): Promise<ApplicationAccessResponse> {
    return request<ApplicationAccessResponse>(
      `/api/v1/workspaces/${encodeURIComponent(workspaceSlug)}/applications/${encodeURIComponent(applicationID)}/access`, token,
    );
  },

  setApplicationAccessMode(workspaceSlug: string, applicationID: string, mode: ApplicationAccessResponse["accessMode"], token: string | null): Promise<ApplicationAccessResponse> {
    return request<ApplicationAccessResponse>(
      `/api/v1/workspaces/${encodeURIComponent(workspaceSlug)}/applications/${encodeURIComponent(applicationID)}/access`,
      token, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ accessMode: mode }) },
    );
  },

  grantApplicationAccess(workspaceSlug: string, applicationID: string, userID: string, role: "editor" | "viewer", token: string | null): Promise<ApplicationAccessResponse> {
    return request<ApplicationAccessResponse>(
      `/api/v1/workspaces/${encodeURIComponent(workspaceSlug)}/applications/${encodeURIComponent(applicationID)}/access/${encodeURIComponent(userID)}`,
      token, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ role }) },
    );
  },

  revokeApplicationAccess(workspaceSlug: string, applicationID: string, userID: string, token: string | null): Promise<ApplicationAccessResponse> {
    return request<ApplicationAccessResponse>(
      `/api/v1/workspaces/${encodeURIComponent(workspaceSlug)}/applications/${encodeURIComponent(applicationID)}/access/${encodeURIComponent(userID)}`,
      token, { method: "DELETE" },
    );
  },
};

export const api = {
  ...mockApi,
  ...applicationAPI,
};
