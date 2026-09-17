import { mockApi } from "@/lib/api/mock-api";
import type { Application, AppStatus } from "@/types/domain";

type ApplicationLifecycle = "active" | "paused" | "archived";

interface ApplicationResponse {
  id: string;
  name: string;
  slug: string;
  description: string;
  lifecycle: ApplicationLifecycle;
  defaultHostname: string;
  createdAt: string;
  updatedAt: string;
}

interface ApplicationListResponse {
  applications: ApplicationResponse[];
}

interface ErrorResponse {
  error?: {
    code?: string;
    message?: string;
    requestId?: string;
  };
}

const apiURL = process.env.NEXT_PUBLIC_SPROUT_API_URL ?? "http://127.0.0.1:8080";
const developmentUserID =
  process.env.NEXT_PUBLIC_SPROUT_DEVELOPMENT_USER_ID ??
  (process.env.NODE_ENV === "development" ? "11111111-1111-4111-8111-111111111111" : "");
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

async function request<T>(path: string): Promise<T> {
  if (!developmentUserID) {
    throw new APIError(
      "The temporary development identity is not configured.",
      0,
      "client_configuration_error",
    );
  }

  const response = await fetch(`${apiURL}${path}`, {
    headers: {
      Accept: "application/json",
      "X-Sprout-User-ID": developmentUserID,
    },
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
    status: applicationStatus(application.lifecycle),
    url: application.defaultHostname,
    updatedAt: updatedLabel(application.updatedAt),
    accent: applicationAccent(application.id),
  };
}

const applicationAPI = {
  async getApplications(workspaceSlug: string): Promise<Application[]> {
    const response = await request<ApplicationListResponse>(
      `/api/v1/workspaces/${encodeURIComponent(workspaceSlug)}/applications`,
    );
    return response.applications.map(mapApplication);
  },

  async getApplication(workspaceSlug: string, applicationID: string): Promise<Application> {
    const response = await request<ApplicationResponse>(
      `/api/v1/workspaces/${encodeURIComponent(workspaceSlug)}/applications/${encodeURIComponent(applicationID)}`,
    );
    return mapApplication(response);
  },
};

export const api = {
  ...mockApi,
  ...applicationAPI,
};
