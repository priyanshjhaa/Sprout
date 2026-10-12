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
  // The address shown for the app; the live URL when there is one.
  url: string;
  // Set only while the app has a live deployment.
  liveUrl: string | null;
  updatedAt: string;
  accent: string;
  accessMode?: "workspace" | "restricted";
  createdBy?: string;
}

