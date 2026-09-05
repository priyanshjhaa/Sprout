import type {
  AgentEvent,
  Application,
  Deployment,
  EnvironmentVariable,
  LogEntry,
  Member,
  Workspace,
} from "@/types/domain";

const wait = (milliseconds = 260) =>
  new Promise((resolve) => setTimeout(resolve, milliseconds));

const workspace: Workspace = { id: "ws_acme", slug: "acme", name: "Acme studio" };

const applications: Application[] = [
  {
    id: "invoice-approvals",
    name: "Invoice approvals",
    slug: "invoice-approvals",
    description: "Review, route, and approve vendor invoices with a small team.",
    status: "running",
    url: "invoice-acme.sprout.run",
    updatedAt: "12 min ago",
    accent: "mint",
  },
  {
    id: "hiring-pipeline",
    name: "Hiring pipeline",
    slug: "hiring-pipeline",
    description: "A focused recruiting board for the product team.",
    status: "running",
    url: "hiring-acme.sprout.run",
    updatedAt: "Yesterday",
    accent: "amber",
  },
  {
    id: "inventory-tracker",
    name: "Inventory tracker",
    slug: "inventory-tracker",
    description: "Reconcile warehouse counts from weekly CSV uploads.",
    status: "building",
    url: "inventory-acme.sprout.run",
    updatedAt: "Just now",
    accent: "blue",
  },
  {
    id: "research-library",
    name: "Research library",
    slug: "research-library",
    description: "A searchable home for customer calls and product notes.",
    status: "failed",
    url: "research-acme.sprout.run",
    updatedAt: "2 days ago",
    accent: "rose",
  },
];

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

const agentEvents: AgentEvent[] = [
  { id: "1", label: "Created application", detail: "invoice-approvals", state: "complete" },
  { id: "2", label: "Connected database", detail: "PostgreSQL · shared-small", state: "complete" },
  { id: "3", label: "Built deployment", detail: "Image built in 41 seconds", state: "complete" },
  { id: "4", label: "Health check passed", detail: "Application is ready", state: "complete" },
];

export const api = {
  async getWorkspace() {
    await wait();
    return workspace;
  },
  async getApplications() {
    await wait();
    return applications;
  },
  async getApplication(appId: string) {
    await wait();
    const application = applications.find((item) => item.id === appId);
    if (!application) throw new Error("Application not found");
    return application;
  },
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
  async getAgentEvents() {
    await wait();
    return agentEvents;
  },
};

