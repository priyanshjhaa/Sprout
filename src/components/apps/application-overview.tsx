"use client";

import { Archive, ArrowRight, Clock3, Lock, Pause, Rocket, TriangleAlert, Users } from "lucide-react";
import Link from "next/link";
import { useApplication, useDeployments } from "@/lib/query/hooks";
import type { Application } from "@/types/domain";

// One clear next step for each lifecycle state.
function lifecycleSummary(application: Application, base: string) {
  if (application.lifecycle === "paused") {
    return { icon: Pause, title: "Paused.", detail: "Deployments are on hold until you resume it.", action: { label: "Resume in settings", href: `${base}/settings` } };
  }
  if (application.lifecycle === "archived") {
    return { icon: Archive, title: "Archived.", detail: "Kept for reference. Restore it to deploy again.", action: { label: "Restore in settings", href: `${base}/settings` } };
  }
  return { icon: Rocket, title: "Not deployed yet.", detail: "Run a deployment to watch it move through the pipeline.", action: { label: "Open deployments", href: `${base}/deployments` } };
}

export function ApplicationOverview({ workspaceSlug, appId }: { workspaceSlug: string; appId: string }) {
  const { data: application, isLoading, isError } = useApplication(workspaceSlug, appId);
  const { data: deployments, isError: deploymentsError } = useDeployments(workspaceSlug, appId);
  const base = `/workspace/${workspaceSlug}/apps/${appId}`;
  const latest = deployments?.[0];

  if (isLoading) return <div className="overview-loading"><span /><span /></div>;
  if (isError || !application) return <section className="soft-empty"><TriangleAlert size={20} /><h2>Application unavailable</h2><p>Sprout could not load this application with your current access.</p></section>;

  const summary = lifecycleSummary(application, base);
  const restricted = application.accessMode === "restricted";

  return (
    <div className="overview-layout">
      <section className="health-card">
        <div className="health-copy">
          <span className="health-icon"><summary.icon size={18} /></span>
          <div><h2>{summary.title}</h2><span>{summary.detail}</span></div>
        </div>
        <Link className="button button-secondary" href={summary.action.href}>{summary.action.label} <ArrowRight size={14} /></Link>
      </section>

      <div className="overview-grid">
        <section className="overview-card latest-deployment">
          <div className="card-label"><span>Latest deployment</span><Clock3 size={14} /></div>
          <div className="deployment-summary">
            <div>
              <strong>{deploymentsError ? "History unavailable" : latest ? `Simulation ${latest.status}` : "None yet"}</strong>
              {!deploymentsError && latest && <small>{new Date(latest.createdAt).toLocaleString()}</small>}
            </div>
          </div>
          <div className="overview-card-footer"><span /><Link href={`${base}/deployments`}>All deployments <ArrowRight size={13} /></Link></div>
        </section>

        <section className="overview-card">
          <div className="card-label"><span>Who can manage it</span>{restricted ? <Lock size={14} /> : <Users size={14} />}</div>
          <div className="deployment-summary">
            <div>
              <strong>{restricted ? "Only invited people" : "Everyone in this workspace"}</strong>
              <small>{restricted ? "Owners, its creator, and people you grant access." : "By their workspace role."} Anyone can open it once it is live.</small>
            </div>
          </div>
          <div className="overview-card-footer"><span /><Link href={`${base}/access`}>Manage access <ArrowRight size={13} /></Link></div>
        </section>
      </div>
    </div>
  );
}
