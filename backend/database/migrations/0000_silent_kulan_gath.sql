CREATE TYPE "public"."agent_event_type" AS ENUM('application_created', 'source_connected', 'resource_requested', 'deployment_created', 'deployment_live');--> statement-breakpoint
CREATE TYPE "public"."agent_run_status" AS ENUM('queued', 'running', 'completed', 'failed', 'cancelled');--> statement-breakpoint
CREATE TYPE "public"."application_lifecycle" AS ENUM('active', 'paused', 'archived');--> statement-breakpoint
CREATE TYPE "public"."application_role" AS ENUM('editor', 'viewer');--> statement-breakpoint
CREATE TYPE "public"."deployment_stage" AS ENUM('source', 'build', 'package', 'provision', 'release', 'health_check');--> statement-breakpoint
CREATE TYPE "public"."deployment_status" AS ENUM('queued', 'building', 'live', 'failed', 'cancelled');--> statement-breakpoint
CREATE TYPE "public"."operation_status" AS ENUM('pending', 'running', 'succeeded', 'failed', 'skipped');--> statement-breakpoint
CREATE TYPE "public"."resource_status" AS ENUM('provisioning', 'healthy', 'degraded', 'disconnected');--> statement-breakpoint
CREATE TYPE "public"."resource_type" AS ENUM('postgres', 'object_storage');--> statement-breakpoint
CREATE TYPE "public"."source_provider" AS ENUM('github', 'gitlab', 'manual', 'agent');--> statement-breakpoint
CREATE TYPE "public"."variable_classification" AS ENUM('plain', 'secret', 'managed');--> statement-breakpoint
CREATE TYPE "public"."workspace_role" AS ENUM('owner', 'editor', 'viewer');--> statement-breakpoint
CREATE TABLE "agent_run_events" (
	"agent_run_id" uuid NOT NULL,
	"sequence" integer NOT NULL,
	"type" "agent_event_type" NOT NULL,
	"status" "operation_status" DEFAULT 'pending' NOT NULL,
	"application_id" uuid,
	"deployment_id" uuid,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"finished_at" timestamp with time zone,
	CONSTRAINT "agent_run_events_agent_run_id_sequence_pk" PRIMARY KEY("agent_run_id","sequence"),
	CONSTRAINT "agent_run_events_sequence_positive" CHECK ("agent_run_events"."sequence" > 0)
);
--> statement-breakpoint
CREATE TABLE "agent_runs" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"workspace_id" uuid NOT NULL,
	"requested_by" uuid,
	"application_id" uuid,
	"status" "agent_run_status" DEFAULT 'queued' NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"started_at" timestamp with time zone,
	"finished_at" timestamp with time zone
);
--> statement-breakpoint
CREATE TABLE "application_access_grants" (
	"application_id" uuid NOT NULL,
	"user_id" uuid NOT NULL,
	"role" "application_role" NOT NULL,
	"granted_by" uuid,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "application_access_grants_application_id_user_id_pk" PRIMARY KEY("application_id","user_id")
);
--> statement-breakpoint
CREATE TABLE "application_resources" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"application_id" uuid NOT NULL,
	"type" "resource_type" NOT NULL,
	"status" "resource_status" DEFAULT 'provisioning' NOT NULL,
	"plan" varchar(80) NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "applications" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"workspace_id" uuid NOT NULL,
	"name" varchar(120) NOT NULL,
	"slug" varchar(63) NOT NULL,
	"description" text DEFAULT '' NOT NULL,
	"lifecycle" "application_lifecycle" DEFAULT 'active' NOT NULL,
	"default_hostname" varchar(253) NOT NULL,
	"created_by" uuid NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "applications_slug_format" CHECK ("applications"."slug" ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$')
);
--> statement-breakpoint
CREATE TABLE "deployment_stage_events" (
	"deployment_id" uuid NOT NULL,
	"stage" "deployment_stage" NOT NULL,
	"status" "operation_status" DEFAULT 'pending' NOT NULL,
	"failure_code" varchar(80),
	"started_at" timestamp with time zone,
	"finished_at" timestamp with time zone,
	CONSTRAINT "deployment_stage_events_deployment_id_stage_pk" PRIMARY KEY("deployment_id","stage")
);
--> statement-breakpoint
CREATE TABLE "deployments" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"application_id" uuid NOT NULL,
	"source_connection_id" uuid,
	"triggered_by" uuid,
	"status" "deployment_status" DEFAULT 'queued' NOT NULL,
	"branch" varchar(255),
	"commit_sha" varchar(64),
	"failure_code" varchar(80),
	"duration_ms" integer,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"started_at" timestamp with time zone,
	"finished_at" timestamp with time zone,
	CONSTRAINT "deployments_duration_nonnegative" CHECK ("deployments"."duration_ms" is null or "deployments"."duration_ms" >= 0)
);
--> statement-breakpoint
CREATE TABLE "environment_variable_metadata" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"application_id" uuid NOT NULL,
	"key" varchar(128) NOT NULL,
	"classification" "variable_classification" DEFAULT 'plain' NOT NULL,
	"required" boolean DEFAULT false NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "environment_variable_metadata_key_format" CHECK ("environment_variable_metadata"."key" ~ '^[A-Z_][A-Z0-9_]*$')
);
--> statement-breakpoint
CREATE TABLE "source_connections" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"application_id" uuid NOT NULL,
	"provider" "source_provider" NOT NULL,
	"repository_owner" varchar(120),
	"repository_name" varchar(180),
	"default_branch" varchar(255) DEFAULT 'main' NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "source_connections_repository_identity" CHECK ("source_connections"."provider" in ('manual', 'agent') or ("source_connections"."repository_owner" is not null and "source_connections"."repository_name" is not null))
);
--> statement-breakpoint
CREATE TABLE "users" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"auth_provider" varchar(32) NOT NULL,
	"auth_subject" varchar(255) NOT NULL,
	"email" varchar(320) NOT NULL,
	"display_name" varchar(120) NOT NULL,
	"avatar_url" text,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "workspace_memberships" (
	"workspace_id" uuid NOT NULL,
	"user_id" uuid NOT NULL,
	"role" "workspace_role" NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "workspace_memberships_workspace_id_user_id_pk" PRIMARY KEY("workspace_id","user_id")
);
--> statement-breakpoint
CREATE TABLE "workspaces" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"slug" varchar(63) NOT NULL,
	"name" varchar(120) NOT NULL,
	"created_by" uuid NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "workspaces_slug_format" CHECK ("workspaces"."slug" ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$')
);
--> statement-breakpoint
ALTER TABLE "agent_run_events" ADD CONSTRAINT "agent_run_events_agent_run_id_agent_runs_id_fk" FOREIGN KEY ("agent_run_id") REFERENCES "public"."agent_runs"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "agent_run_events" ADD CONSTRAINT "agent_run_events_application_id_applications_id_fk" FOREIGN KEY ("application_id") REFERENCES "public"."applications"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "agent_run_events" ADD CONSTRAINT "agent_run_events_deployment_id_deployments_id_fk" FOREIGN KEY ("deployment_id") REFERENCES "public"."deployments"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "agent_runs" ADD CONSTRAINT "agent_runs_workspace_id_workspaces_id_fk" FOREIGN KEY ("workspace_id") REFERENCES "public"."workspaces"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "agent_runs" ADD CONSTRAINT "agent_runs_requested_by_users_id_fk" FOREIGN KEY ("requested_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "agent_runs" ADD CONSTRAINT "agent_runs_application_id_applications_id_fk" FOREIGN KEY ("application_id") REFERENCES "public"."applications"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "application_access_grants" ADD CONSTRAINT "application_access_grants_application_id_applications_id_fk" FOREIGN KEY ("application_id") REFERENCES "public"."applications"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "application_access_grants" ADD CONSTRAINT "application_access_grants_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "application_access_grants" ADD CONSTRAINT "application_access_grants_granted_by_users_id_fk" FOREIGN KEY ("granted_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "application_resources" ADD CONSTRAINT "application_resources_application_id_applications_id_fk" FOREIGN KEY ("application_id") REFERENCES "public"."applications"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "applications" ADD CONSTRAINT "applications_workspace_id_workspaces_id_fk" FOREIGN KEY ("workspace_id") REFERENCES "public"."workspaces"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "applications" ADD CONSTRAINT "applications_created_by_users_id_fk" FOREIGN KEY ("created_by") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "deployment_stage_events" ADD CONSTRAINT "deployment_stage_events_deployment_id_deployments_id_fk" FOREIGN KEY ("deployment_id") REFERENCES "public"."deployments"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "deployments" ADD CONSTRAINT "deployments_application_id_applications_id_fk" FOREIGN KEY ("application_id") REFERENCES "public"."applications"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "deployments" ADD CONSTRAINT "deployments_source_connection_id_source_connections_id_fk" FOREIGN KEY ("source_connection_id") REFERENCES "public"."source_connections"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "deployments" ADD CONSTRAINT "deployments_triggered_by_users_id_fk" FOREIGN KEY ("triggered_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "environment_variable_metadata" ADD CONSTRAINT "environment_variable_metadata_application_id_applications_id_fk" FOREIGN KEY ("application_id") REFERENCES "public"."applications"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "source_connections" ADD CONSTRAINT "source_connections_application_id_applications_id_fk" FOREIGN KEY ("application_id") REFERENCES "public"."applications"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "workspace_memberships" ADD CONSTRAINT "workspace_memberships_workspace_id_workspaces_id_fk" FOREIGN KEY ("workspace_id") REFERENCES "public"."workspaces"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "workspace_memberships" ADD CONSTRAINT "workspace_memberships_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "workspaces" ADD CONSTRAINT "workspaces_created_by_users_id_fk" FOREIGN KEY ("created_by") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
CREATE INDEX "agent_runs_workspace_created_idx" ON "agent_runs" USING btree ("workspace_id","created_at");--> statement-breakpoint
CREATE INDEX "agent_runs_status_idx" ON "agent_runs" USING btree ("status");--> statement-breakpoint
CREATE INDEX "application_access_grants_user_idx" ON "application_access_grants" USING btree ("user_id");--> statement-breakpoint
CREATE INDEX "application_resources_application_idx" ON "application_resources" USING btree ("application_id");--> statement-breakpoint
CREATE UNIQUE INDEX "applications_workspace_slug_unique" ON "applications" USING btree ("workspace_id","slug");--> statement-breakpoint
CREATE UNIQUE INDEX "applications_default_hostname_unique" ON "applications" USING btree ("default_hostname");--> statement-breakpoint
CREATE INDEX "applications_workspace_updated_idx" ON "applications" USING btree ("workspace_id","updated_at");--> statement-breakpoint
CREATE INDEX "deployment_stage_events_status_idx" ON "deployment_stage_events" USING btree ("status");--> statement-breakpoint
CREATE INDEX "deployments_application_created_idx" ON "deployments" USING btree ("application_id","created_at");--> statement-breakpoint
CREATE INDEX "deployments_status_idx" ON "deployments" USING btree ("status");--> statement-breakpoint
CREATE UNIQUE INDEX "environment_variable_metadata_app_key_unique" ON "environment_variable_metadata" USING btree ("application_id","key");--> statement-breakpoint
CREATE UNIQUE INDEX "source_connections_application_unique" ON "source_connections" USING btree ("application_id");--> statement-breakpoint
CREATE UNIQUE INDEX "users_auth_identity_unique" ON "users" USING btree ("auth_provider","auth_subject");--> statement-breakpoint
CREATE UNIQUE INDEX "users_email_lower_unique" ON "users" USING btree (lower("email"));--> statement-breakpoint
CREATE INDEX "workspace_memberships_user_idx" ON "workspace_memberships" USING btree ("user_id");--> statement-breakpoint
CREATE UNIQUE INDEX "workspaces_slug_unique" ON "workspaces" USING btree ("slug");