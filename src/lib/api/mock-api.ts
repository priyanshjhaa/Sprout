import type {
  Deployment,
  EnvironmentVariable,
  LogEntry,
  Member,
} from "@/types/domain";

const wait = (milliseconds = 260) =>
  new Promise((resolve) => setTimeout(resolve, milliseconds));

const deployments: Deployment[] = [
  {
    id: "dep_104",
    appId: "invoice-approvals",
    status: "live",
    branch: "main",
    commit: "a81f29c",
    message: "Add approval notes",
    createdAt: "12 min ago",
    duration: "48s",
  },
  {
    id: "dep_103",
    appId: "invoice-approvals",
    status: "live",
    branch: "main",
    commit: "bb724e1",
    message: "Improve invoice table",
    createdAt: "Yesterday",
    duration: "51s",
  },
  {
    id: "dep_102",
    appId: "invoice-approvals",
    status: "failed",
    branch: "main",
    commit: "c0a871d",
    message: "Update database client",
    createdAt: "2 days ago",
    duration: "32s",
  },
];

const logs: LogEntry[] = [
  { id: "1", timestamp: "10:42:02", level: "info", method: "GET", message: "/api/invoices 200 · 38ms" },
  { id: "2", timestamp: "10:42:04", level: "info", method: "POST", message: "/api/approval 201 · 91ms" },
  { id: "3", timestamp: "10:42:07", level: "warn", message: "Database pool nearing connection limit" },
  { id: "4", timestamp: "10:42:11", level: "info", method: "GET", message: "/api/team 200 · 24ms" },
  { id: "5", timestamp: "10:42:16", level: "error", message: "Invoice extraction timed out; retry scheduled" },
  { id: "6", timestamp: "10:42:19", level: "info", message: "Retry completed successfully" },
];

const environment: EnvironmentVariable[] = [
  { key: "DATABASE_URL", value: "Managed by Sprout", managed: true, secret: true },
  { key: "OPENAI_API_KEY", value: "sk-sprout-demo", managed: false, secret: true },
  { key: "APP_NAME", value: "Invoice approvals", managed: false, secret: false },
];

const members: Member[] = [
  { id: "1", name: "Priyansh Jha", email: "priyansh@acme.test", role: "Owner", initials: "PJ" },
  { id: "2", name: "Alice Chen", email: "alice@acme.test", role: "Editor", initials: "AC" },
  { id: "3", name: "Bob Hart", email: "bob@acme.test", role: "Viewer", initials: "BH" },
];


export const mockApi = {
  async getDeployments(appId: string) {
    await wait();
    return deployments.filter((deployment) => deployment.appId === appId);
  },
  async getLogs() {
    await wait(180);
    return logs;
  },
  async getEnvironment() {
    await wait();
    return environment;
  },
  async getMembers() {
    await wait();
    return members;
  },
};
