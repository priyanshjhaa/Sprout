ALTER TYPE "public"."deployment_status" ADD VALUE 'succeeded';--> statement-breakpoint
ALTER TABLE "deployments" ADD COLUMN "simulated" boolean DEFAULT false NOT NULL;--> statement-breakpoint
CREATE UNIQUE INDEX "deployments_one_active_simulation_per_app" ON "deployments" USING btree ("application_id") WHERE "deployments"."simulated" = true and "deployments"."status" in ('queued', 'building');--> statement-breakpoint
ALTER TABLE "deployments" ADD CONSTRAINT "deployments_simulation_not_live" CHECK (not "deployments"."simulated" or "deployments"."status" <> 'live');