// "undeployed" is an active application that has no real deployment yet.
export type AppStatus = "undeployed" | "running" | "building" | "failed" | "paused" | "archived";
export type ApplicationLifecycle = "active" | "paused" | "archived";

export interface Workspace {
  id: string;
  slug: string;
  name: string;
}

export interface Application {
  id: string;
  name: string;
  slug: string;
  description: string;
  status: AppStatus;
  lifecycle: ApplicationLifecycle;
  url: string;
  updatedAt: string;
  accent: string;
  accessMode?: "workspace" | "restricted";
  createdBy?: string;
}

