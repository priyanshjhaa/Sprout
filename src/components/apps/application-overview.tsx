"use client";

import { Archive, ArrowRight, Clock3, ExternalLink, Globe, Lock, Pause, Rocket, TriangleAlert, Users } from "lucide-react";
import Link from "next/link";
import { StatusPill } from "@/components/status-pill";
import { useApplication } from "@/lib/query/hooks";
import { useLatestDeployment } from "@/lib/query/simulations";
import type { Application } from "@/types/domain";

// One clear next step for each lifecycle state.
function lifecycleSummary(application: Application, base: string) {
  if (application.lifecycle === "paused") {
    return { icon: Pause, title: "Paused.", detail: "Its URL is offline and deployments are on hold until you resume it.", action: { label: "Resume in settings", href: `${base}/settings` } };
  }
  if (application.lifecycle === "archived") {
    return { icon: Archive, title: "Archived.", detail: "Its URL is offline. Restore it, then deploy again.", action: { label: "Restore in settings", href: `${base}/settings` } };
  }
  if (application.liveUrl) {
    return { icon: Globe, title: "Live.", detail: "Anyone with the link can open it. It sleeps when idle and wakes on the next visit.", action: { label: "Open the app", href: application.liveUrl, external: true } };
  }
  return { icon: Rocket, title: "Not deployed yet.", detail: "Run a deployment to watch it move through the pipeline.", action: { label: "Open deployments", href: `${base}/deployments` } };
}

export function ApplicationOverview({ workspaceSlug, appId }: { workspaceSlug: string; appId: string }) {
  const { data: application, isLoading, isError } = useApplication(workspaceSlug, appId);
  const { job: latest, kind, isError: deploymentsError } = useLatestDeployment(workspaceSlug, appId);
  const base = `/workspace/${workspaceSlug}/apps/${appId}`;

  if (isLoading) return <div className="overview-loading"><span /><span /></div>;
  if (isError || !application) return <section className="soft-empty"><TriangleAlert size={20} /><h2>Application unavailable</h2><p>Sprout could not load this application with your current access.</p></section>;

  const summary = lifecycleSummary(application, base);
  const restricted = application.accessMode === "restricted";

  return (
    <div className="sp-overview">
      <section className="sp-next">
        <span className="sp-next-icon"><summary.icon size={20} /></span>
        <div><h2>{summary.title}</h2><p>{summary.detail}</p></div>
        {"external" in summary.action
          ? <a className="sp-button sp-button-solid" href={summary.action.href} target="_blank" rel="noreferrer">{summary.action.label} <ExternalLink size={14} /></a>
          : <Link className="sp-button sp-button-solid" href={summary.action.href}>{summary.action.label} <ArrowRight size={14} /></Link>}
      </section>

      <div className="sp-overview-grid">
        <section className="sp-panel sp-fact">
          <div className="sp-fact-label"><span>Latest deployment</span><Clock3 size={14} /></div>
          <strong>{deploymentsError ? "History unavailable" : latest ? `${kind === "build" ? "Build" : "Simulation"} ${latest.status}` : "None yet"}</strong>
          {!deploymentsError && latest && <small>{new Date(latest.createdAt).toLocaleString()}</small>}
          <div className="sp-fact-foot">{latest && <StatusPill status={latest.status} />}<Link href={latest ? `${base}/deployments/${latest.id}${kind === "build" ? "?kind=build" : ""}` : `${base}/deployments`}>{latest ? "Open it" : "Deployments"} <ArrowRight size={13} /></Link></div>
        </section>

        <section className="sp-panel sp-fact">
          <div className="sp-fact-label"><span>Who can manage it</span>{restricted ? <Lock size={14} /> : <Users size={14} />}</div>
          <strong>{restricted ? "Only invited people" : "Everyone in this workspace"}</strong>
          <small>{restricted ? "Owners, its creator, and people you grant access." : "By their workspace role."} Anyone can open it once it is live.</small>
          <div className="sp-fact-foot"><span /><Link href={`${base}/access`}>Manage access <ArrowRight size={13} /></Link></div>
        </section>
      </div>
    </div>
  );
}
