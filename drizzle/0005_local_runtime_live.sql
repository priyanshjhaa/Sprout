ALTER TABLE "deployments" DROP CONSTRAINT "deployments_artifact_reference";--> statement-breakpoint
CREATE UNIQUE INDEX "deployments_one_live_per_app" ON "deployments" USING btree ("application_id") WHERE "deployments"."status" = 'live';--> statement-breakpoint
ALTER TABLE "deployments" ADD CONSTRAINT "deployments_artifact_reference" CHECK (("deployments"."artifact_id" is null and "deployments"."artifact_sha256" is null and "deployments"."artifact_bytes" is null)
        or (not "deployments"."simulated" and "deployments"."status" in ('succeeded', 'live')
          and "deployments"."artifact_id" is not null and "deployments"."artifact_id" ~ '^[0-9a-f]{32}$'
          and "deployments"."artifact_sha256" is not null and "deployments"."artifact_sha256" ~ '^[0-9a-f]{64}$'
          and "deployments"."artifact_bytes" is not null and "deployments"."artifact_bytes" between 1 and 16777216));