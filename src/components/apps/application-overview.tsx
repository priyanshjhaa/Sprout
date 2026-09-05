"use client";

import { ArrowRight, Check, Clock3, Database, ExternalLink, HardDrive, TriangleAlert } from "lucide-react";
import Link from "next/link";
import { useApplication, useDeployments } from "@/lib/query/hooks";

export function ApplicationOverview({ workspaceSlug, appId }: { workspaceSlug: string; appId: string }) {
  const { data: application, isLoading, isError } = useApplication(workspaceSlug, appId);
  const { data: deployments } = useDeployments(workspaceSlug, appId);
  const latest = deployments?.[0];

  if (isLoading) return <div className="overview-loading"><span /><span /><span /></div>;
  if (isError || !application) return <section className="soft-empty"><TriangleAlert size={20} /><h2>Application unavailable</h2><p>Sprout could not find this application in the demo workspace.</p></section>;

  return (
    <div className="overview-layout">
      <section className="health-card">
        <div className="health-copy">
          <span className="health-icon"><Check size={18} /></span>
          <div><p>Application health</p><h2>Your app is running quietly.</h2><span>All health checks are passing. Nothing needs your attention.</span></div>
        </div>
        <a href={`https://${application.url}`} target="_blank" rel="noreferrer">{application.url}<ExternalLink size={13} /></a>
      </section>

      <div className="overview-grid">
        <section className="overview-card latest-deployment">
          <div className="card-label"><span>Latest deployment</span><Clock3 size={14} /></div>
          <div className="deployment-summary"><span className="success-check"><Check size={13} /></span><div><strong>{latest?.message ?? "No deployment yet"}</strong><small>{latest ? `${latest.branch} · ${latest.commit}` : "—"}</small></div></div>
          <div className="overview-card-footer"><span>{latest?.createdAt} · {latest?.duration}</span><Link href={`/workspace/${workspaceSlug}/apps/${appId}/deployments`}>View deployments <ArrowRight size={13} /></Link></div>
        </section>

        <section className="overview-card resources-summary">
          <div className="card-label"><span>Resources</span><span>2 connected</span></div>
          <div className="resource-summary-row"><span><Database size={15} /></span><div><strong>PostgreSQL</strong><small>Connected and healthy</small></div><i /></div>
          <div className="resource-summary-row"><span><HardDrive size={15} /></span><div><strong>Object storage</strong><small>Not configured</small></div><button type="button">Add</button></div>
        </section>
      </div>

      <section className="overview-card recent-note">
        <div><p>Nothing needs attention</p><span>Sprout will surface failed deployments, disconnected resources, and access changes here.</span></div>
        <span className="quiet-pulse"><i /></span>
      </section>
    </div>
  );
}

