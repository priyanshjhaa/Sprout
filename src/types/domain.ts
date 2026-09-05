export type AppStatus = "running" | "building" | "failed" | "paused";
export type DeploymentStatus = "live" | "building" | "failed";

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
  url: string;
  updatedAt: string;
  accent: string;
}

export interface Deployment {
  id: string;
  appId: string;
  status: DeploymentStatus;
  branch: string;
  commit: string;
  message: string;
  createdAt: string;
  duration: string;
}

export interface LogEntry {
  id: string;
  timestamp: string;
  level: "info" | "warn" | "error";
  method?: string;
  message: string;
}

export interface EnvironmentVariable {
  key: string;
  value: string;
  managed: boolean;
  secret: boolean;
}

export interface Member {
  id: string;
  name: string;
  email: string;
  role: "Owner" | "Editor" | "Viewer";
  initials: string;
}

export interface AgentEvent {
  id: string;
  label: string;
  detail: string;
  state: "complete" | "active" | "pending";
}

