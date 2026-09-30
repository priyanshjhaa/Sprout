"use client";

import { ArrowRight, FlaskConical, Play } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { StatusPill } from "@/components/status-pill";
import { useApplication, useDeployments } from "@/lib/query/hooks";
import { useSimulationActions } from "@/lib/query/simulations";
import { isActiveSimulation } from "@/lib/api/simulation-stream";

export function DeploymentsView({ workspaceSlug, appId }: { workspaceSlug: string; appId: string }) {
  const query = useDeployments(workspaceSlug, appId);
  const application = useApplication(workspaceSlug, appId);
  const action = useSimulationActions(workspaceSlug, appId);
  const router = useRouter();
  const base = `/workspace/${workspaceSlug}/apps/${appId}/deployments`;
  const busy = query.data?.some(isActiveSimulation);
  const canStart = application.data?.status === "running" && query.isSuccess && !busy && !action.isPending;

  return (
    <section className="app-section simulation-view">
      <header className="section-heading">
        <div><h2>Deployment simulations</h2><p>Practice the deployment flow. No application code runs.</p></div>
        <button className="button button-secondary" type="button" disabled={!canStart}
          onClick={() => action.mutate(undefined, { onSuccess: (job) => router.push(`${base}/${job.id}`) })}>
          <Play size={14} /> {action.isPending ? "Starting…" : "Run simulation"}
        </button>
      </header>
      <p className="simulation-note">Application edit access is required to run or cancel. Simulations never create a live URL. Only one can run per application.</p>
      {application.data && application.data.status !== "running" && <p className="simulation-note">Resume this application before running a simulation.</p>}
      {action.isError && <p className="simulation-notice" role="alert">{action.error.message} Check the list before trying again.</p>}
      {query.isPending && <p className="simulation-notice" role="status">Loading simulations…</p>}
      {query.isError && <div className="simulation-notice" role="alert"><p>{query.error.message}</p><button className="button button-secondary" onClick={() => void query.refetch()}>Try again</button></div>}
      {query.isSuccess && query.data.length === 0 && <div className="soft-empty"><FlaskConical size={22} /><h3>No simulations yet</h3><p>Run one to see the worker move through source, build, and package steps.</p></div>}
      {query.isSuccess && query.data.length > 0 && <div className="deployment-list">
        {query.data.map((job) => (
          <Link className="deployment-row" href={`${base}/${job.id}`} key={job.id}>
            <span className="deployment-icon"><FlaskConical size={15} /></span>
            <div className="deployment-main"><div><strong>Simulation {job.id.slice(0, 8)}</strong><span className="current-label">Simulated</span></div><span>{job.stages.filter((stage) => stage.status === "succeeded").length} of 3 steps complete</span></div>
            <StatusPill status={job.status} />
            <div className="deployment-time"><strong>{new Date(job.createdAt).toLocaleString()}</strong></div>
            <ArrowRight className="deployment-arrow" size={14} />
          </Link>
        ))}
      </div>}
      {query.isSuccess && <p className="simulation-note">Showing the latest 50 simulations. <button className="simulation-refresh" type="button" disabled={query.isFetching} onClick={() => void query.refetch()}>Refresh history</button></p>}
    </section>
  );
}
