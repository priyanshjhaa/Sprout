"use client";

import { ArrowRight, Check, Clock3, Database, HardDrive, TriangleAlert } from "lucide-react";
import Link from "next/link";
import { useApplication, useDeployments } from "@/lib/query/hooks";

export function ApplicationOverview({ workspaceSlug, appId }: { workspaceSlug: string; appId: string }) {
  const { data: application, isLoading, isError } = useApplication(workspaceSlug, appId);
  const { data: deployments, isError: deploymentsError } = useDeployments(workspaceSlug, appId);
  const latest = deployments?.[0];

  if (isLoading) return <div className="overview-loading"><span /><span /><span /></div>;
  if (isError || !application) return <section className="soft-empty"><TriangleAlert size={20} /><h2>Application unavailable</h2><p>Sprout could not load this application with your current access.</p></section>;

  return (
    <div className="overview-layout">
      <section className="health-card">
        <div className="health-copy">
          <span className="health-icon"><Check size={18} /></span>
          <div><p>Application runtime</p><h2>Ready for its first real deployment.</h2><span>Deployment simulations are available. Runtime health checks are not connected yet.</span></div>
        </div>
        <span>Proposed address: {application.url}</span>
      </section>

      <div className="overview-grid">
        <section className="overview-card latest-deployment">
          <div className="card-label"><span>Latest simulation</span><Clock3 size={14} /></div>
          <div className="deployment-summary"><span><Clock3 size={13} /></span><div><strong>{deploymentsError ? "History unavailable" : latest ? `Simulation ${latest.status}` : "No simulation yet"}</strong><small>No application code runs</small></div></div>
          <div className="overview-card-footer"><span>{!deploymentsError && latest ? new Date(latest.createdAt).toLocaleString() : "—"}</span><Link href={`/workspace/${workspaceSlug}/apps/${appId}/deployments`}>View simulations <ArrowRight size={13} /></Link></div>
        </section>

        <section className="overview-card resources-summary">
          <div className="card-label"><span>Resources preview</span><span>Not provisioned</span></div>
          <div className="resource-summary-row"><span><Database size={15} /></span><div><strong>PostgreSQL</strong><small>App database provisioning is not connected yet</small></div></div>
          <div className="resource-summary-row"><span><HardDrive size={15} /></span><div><strong>Object storage</strong><small>Not configured</small></div><button type="button">Add</button></div>
        </section>
      </div>

      <section className="overview-card recent-note">
        <div><p>Runtime monitoring is not connected yet</p><span>For now, open simulations to inspect worker progress and outcomes.</span></div>
        <span className="quiet-pulse"><i /></span>
      </section>
    </div>
  );
}
