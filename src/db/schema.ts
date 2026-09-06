import { sql } from "drizzle-orm";
import {
  boolean,
  check,
  index,
  integer,
  pgEnum,
  pgTable,
  primaryKey,
  text,
  timestamp,
  uniqueIndex,
  uuid,
  varchar,
} from "drizzle-orm/pg-core";

export const workspaceRole = pgEnum("workspace_role", ["owner", "editor", "viewer"]);
export const applicationRole = pgEnum("application_role", ["editor", "viewer"]);
export const applicationLifecycle = pgEnum("application_lifecycle", ["active", "paused", "archived"]);
export const sourceProvider = pgEnum("source_provider", ["github", "gitlab", "manual", "agent"]);
export const deploymentStatus = pgEnum("deployment_status", ["queued", "building", "live", "failed", "cancelled"]);
export const deploymentStage = pgEnum("deployment_stage", [
  "source",
  "build",
  "package",
  "provision",
  "release",
  "health_check",
]);
export const operationStatus = pgEnum("operation_status", ["pending", "running", "succeeded", "failed", "skipped"]);
export const resourceType = pgEnum("resource_type", ["postgres", "object_storage"]);
export const resourceStatus = pgEnum("resource_status", ["provisioning", "healthy", "degraded", "disconnected"]);
export const variableClassification = pgEnum("variable_classification", ["plain", "secret", "managed"]);
export const agentRunStatus = pgEnum("agent_run_status", ["queued", "running", "completed", "failed", "cancelled"]);
export const agentEventType = pgEnum("agent_event_type", [
  "application_created",
  "source_connected",
  "resource_requested",
  "deployment_created",
  "deployment_live",
]);

const timestamps = () => ({
  createdAt: timestamp("created_at", { withTimezone: true }).defaultNow().notNull(),
  updatedAt: timestamp("updated_at", { withTimezone: true }).defaultNow().notNull(),
});

export const users = pgTable(
  "users",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    authProvider: varchar("auth_provider", { length: 32 }).notNull(),
    authSubject: varchar("auth_subject", { length: 255 }).notNull(),
    email: varchar("email", { length: 320 }).notNull(),
    displayName: varchar("display_name", { length: 120 }).notNull(),
    avatarUrl: text("avatar_url"),
    ...timestamps(),
  },
  (table) => [
    uniqueIndex("users_auth_identity_unique").on(table.authProvider, table.authSubject),
    uniqueIndex("users_email_lower_unique").on(sql`lower(${table.email})`),
  ],
);

export const workspaces = pgTable(
  "workspaces",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    slug: varchar("slug", { length: 63 }).notNull(),
    name: varchar("name", { length: 120 }).notNull(),
    createdBy: uuid("created_by").notNull().references(() => users.id, { onDelete: "restrict" }),
    ...timestamps(),
  },
  (table) => [
    uniqueIndex("workspaces_slug_unique").on(table.slug),
    check("workspaces_slug_format", sql`${table.slug} ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'`),
  ],
);

export const workspaceMemberships = pgTable(
  "workspace_memberships",
  {
    workspaceId: uuid("workspace_id").notNull().references(() => workspaces.id, { onDelete: "cascade" }),
    userId: uuid("user_id").notNull().references(() => users.id, { onDelete: "cascade" }),
    role: workspaceRole("role").notNull(),
    createdAt: timestamp("created_at", { withTimezone: true }).defaultNow().notNull(),
  },
  (table) => [
    primaryKey({ columns: [table.workspaceId, table.userId] }),
    index("workspace_memberships_user_idx").on(table.userId),
  ],
);

export const applications = pgTable(
  "applications",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    workspaceId: uuid("workspace_id").notNull().references(() => workspaces.id, { onDelete: "cascade" }),
    name: varchar("name", { length: 120 }).notNull(),
    slug: varchar("slug", { length: 63 }).notNull(),
    description: text("description").notNull().default(""),
    lifecycle: applicationLifecycle("lifecycle").default("active").notNull(),
    defaultHostname: varchar("default_hostname", { length: 253 }).notNull(),
    createdBy: uuid("created_by").notNull().references(() => users.id, { onDelete: "restrict" }),
    ...timestamps(),
  },
  (table) => [
    uniqueIndex("applications_workspace_slug_unique").on(table.workspaceId, table.slug),
    uniqueIndex("applications_default_hostname_unique").on(table.defaultHostname),
    index("applications_workspace_updated_idx").on(table.workspaceId, table.updatedAt),
    check("applications_slug_format", sql`${table.slug} ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'`),
  ],
);

export const sourceConnections = pgTable(
  "source_connections",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    applicationId: uuid("application_id").notNull().references(() => applications.id, { onDelete: "cascade" }),
    provider: sourceProvider("provider").notNull(),
    repositoryOwner: varchar("repository_owner", { length: 120 }),
    repositoryName: varchar("repository_name", { length: 180 }),
    defaultBranch: varchar("default_branch", { length: 255 }).default("main").notNull(),
    ...timestamps(),
  },
  (table) => [
    uniqueIndex("source_connections_application_unique").on(table.applicationId),
    check(
      "source_connections_repository_identity",
      sql`${table.provider} in ('manual', 'agent') or (${table.repositoryOwner} is not null and ${table.repositoryName} is not null)`,
    ),
  ],
);

export const deployments = pgTable(
  "deployments",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    applicationId: uuid("application_id").notNull().references(() => applications.id, { onDelete: "cascade" }),
    sourceConnectionId: uuid("source_connection_id").references(() => sourceConnections.id, { onDelete: "set null" }),
    triggeredBy: uuid("triggered_by").references(() => users.id, { onDelete: "set null" }),
    status: deploymentStatus("status").default("queued").notNull(),
    branch: varchar("branch", { length: 255 }),
    commitSha: varchar("commit_sha", { length: 64 }),
    failureCode: varchar("failure_code", { length: 80 }),
    durationMs: integer("duration_ms"),
    createdAt: timestamp("created_at", { withTimezone: true }).defaultNow().notNull(),
    startedAt: timestamp("started_at", { withTimezone: true }),
    finishedAt: timestamp("finished_at", { withTimezone: true }),
  },
  (table) => [
    index("deployments_application_created_idx").on(table.applicationId, table.createdAt),
    index("deployments_status_idx").on(table.status),
    check("deployments_duration_nonnegative", sql`${table.durationMs} is null or ${table.durationMs} >= 0`),
  ],
);

export const deploymentStageEvents = pgTable(
  "deployment_stage_events",
  {
    deploymentId: uuid("deployment_id").notNull().references(() => deployments.id, { onDelete: "cascade" }),
    stage: deploymentStage("stage").notNull(),
    status: operationStatus("status").default("pending").notNull(),
    failureCode: varchar("failure_code", { length: 80 }),
    startedAt: timestamp("started_at", { withTimezone: true }),
    finishedAt: timestamp("finished_at", { withTimezone: true }),
  },
  (table) => [
    primaryKey({ columns: [table.deploymentId, table.stage] }),
    index("deployment_stage_events_status_idx").on(table.status),
  ],
);

export const applicationResources = pgTable(
  "application_resources",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    applicationId: uuid("application_id").notNull().references(() => applications.id, { onDelete: "cascade" }),
    type: resourceType("type").notNull(),
    status: resourceStatus("status").default("provisioning").notNull(),
    plan: varchar("plan", { length: 80 }).notNull(),
    ...timestamps(),
  },
  (table) => [index("application_resources_application_idx").on(table.applicationId)],
);

export const environmentVariableMetadata = pgTable(
  "environment_variable_metadata",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    applicationId: uuid("application_id").notNull().references(() => applications.id, { onDelete: "cascade" }),
    key: varchar("key", { length: 128 }).notNull(),
    classification: variableClassification("classification").default("plain").notNull(),
    required: boolean("required").default(false).notNull(),
    ...timestamps(),
  },
  (table) => [
    uniqueIndex("environment_variable_metadata_app_key_unique").on(table.applicationId, table.key),
    check("environment_variable_metadata_key_format", sql`${table.key} ~ '^[A-Z_][A-Z0-9_]*$'`),
  ],
);

export const applicationAccessGrants = pgTable(
  "application_access_grants",
  {
    applicationId: uuid("application_id").notNull().references(() => applications.id, { onDelete: "cascade" }),
    userId: uuid("user_id").notNull().references(() => users.id, { onDelete: "cascade" }),
    role: applicationRole("role").notNull(),
    grantedBy: uuid("granted_by").references(() => users.id, { onDelete: "set null" }),
    createdAt: timestamp("created_at", { withTimezone: true }).defaultNow().notNull(),
  },
  (table) => [
    primaryKey({ columns: [table.applicationId, table.userId] }),
    index("application_access_grants_user_idx").on(table.userId),
  ],
);

export const agentRuns = pgTable(
  "agent_runs",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    workspaceId: uuid("workspace_id").notNull().references(() => workspaces.id, { onDelete: "cascade" }),
    requestedBy: uuid("requested_by").references(() => users.id, { onDelete: "set null" }),
    applicationId: uuid("application_id").references(() => applications.id, { onDelete: "set null" }),
    status: agentRunStatus("status").default("queued").notNull(),
    createdAt: timestamp("created_at", { withTimezone: true }).defaultNow().notNull(),
    startedAt: timestamp("started_at", { withTimezone: true }),
    finishedAt: timestamp("finished_at", { withTimezone: true }),
  },
  (table) => [
    index("agent_runs_workspace_created_idx").on(table.workspaceId, table.createdAt),
    index("agent_runs_status_idx").on(table.status),
  ],
);

export const agentRunEvents = pgTable(
  "agent_run_events",
  {
    agentRunId: uuid("agent_run_id").notNull().references(() => agentRuns.id, { onDelete: "cascade" }),
    sequence: integer("sequence").notNull(),
    type: agentEventType("type").notNull(),
    status: operationStatus("status").default("pending").notNull(),
    applicationId: uuid("application_id").references(() => applications.id, { onDelete: "set null" }),
    deploymentId: uuid("deployment_id").references(() => deployments.id, { onDelete: "set null" }),
    createdAt: timestamp("created_at", { withTimezone: true }).defaultNow().notNull(),
    finishedAt: timestamp("finished_at", { withTimezone: true }),
  },
  (table) => [
    primaryKey({ columns: [table.agentRunId, table.sequence] }),
    check("agent_run_events_sequence_positive", sql`${table.sequence} > 0`),
  ],
);

export type User = typeof users.$inferSelect;
export type Workspace = typeof workspaces.$inferSelect;
export type Application = typeof applications.$inferSelect;
export type Deployment = typeof deployments.$inferSelect;
export type AgentRun = typeof agentRuns.$inferSelect;
