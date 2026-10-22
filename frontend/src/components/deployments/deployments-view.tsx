"use client";

import { ArrowRight, FlaskConical, Hammer, Play } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { StatusPill } from "@/components/status-pill";
import { UploadBuild } from "@/components/deployments/upload-build";
import { useApplication, useDeployments } from "@/lib/query/hooks";
import { useBuilds, useCapabilities, useSimulationActions } from "@/lib/query/simulations";
import { isActiveSimulation, type Simulation } from "@/lib/api/simulation-stream";

export function DeploymentsView({ workspaceSlug, appId }: { workspaceSlug: string; appId: string }) {
  const capabilities = useCapabilities();
  const localBuilds = capabilities.data?.localBuilds === true;
  const application = useApplication(workspaceSlug, appId);
  const active = application.data?.lifecycle === "active";

  return (
    <section className="dash-stack">
      <header className="dash-subhead">
        <div>
          <h2>Deployments</h2>
          <p>{localBuilds
            ? "Builds run in a sealed container on this machine. Nothing is started or given a URL yet."
            : "Runs are simulated for now: no code executes and no URL goes live."}</p>
        </div>
      </header>
      {application.data && !active && <p className="dash-callout dash-callout--warn">Resume this application in Settings before deploying.</p>}
      {localBuilds && <Builds workspaceSlug={workspaceSlug} appId={appId} active={active} />}
      <Simulations workspaceSlug={workspaceSlug} appId={appId} active={active} titled={localBuilds} />
    </section>
  );
}

function Builds({ workspaceSlug, appId, active }: { workspaceSlug: string; appId: string; active: boolean }) {
  const query = useBuilds(workspaceSlug, appId, true);
  const busy = query.data?.some(isActiveSimulation);
  return (
    <div className="dash-panel-card">
      <div className="dash-panel-card-head">
        <h3>Builds</h3>
        <UploadBuild workspaceSlug={workspaceSlug} appId={appId} disabled={!active || busy} compact />
      </div>
      <JobList query={query} workspaceSlug={workspaceSlug} appId={appId} kind="build"
        empty="Choose your site's folder above to deploy it." />
    </div>
  );
}

function Simulations({ workspaceSlug, appId, active, titled }: { workspaceSlug: string; appId: string; active: boolean; titled: boolean }) {
  const query = useDeployments(workspaceSlug, appId);
  const action = useSimulationActions(workspaceSlug, appId);
  const router = useRouter();
  const base = `/workspace/${workspaceSlug}/apps/${appId}/deployments`;
  const busy = query.data?.some(isActiveSimulation);
  const canStart = active && query.isSuccess && !busy && !action.isPending;
  return (
    <div className="dash-panel-card">
      <div className="dash-panel-card-head">
        {titled ? <h3>Simulations</h3> : <span />}
        <button className="lp-btn lp-btn--light lp-btn--small" type="button" disabled={!canStart}
          onClick={() => action.mutate(undefined, { onSuccess: (job) => router.push(`${base}/${job.id}`) })}>
          <Play size={14} aria-hidden="true" /> {action.isPending ? "Starting…" : "Run simulation"}
        </button>
      </div>
      {action.isError && <p className="dash-callout dash-callout--error" role="alert">{action.error.message} Check the list before trying again.</p>}
      <JobList query={query} workspaceSlug={workspaceSlug} appId={appId} kind="simulation"
        empty="Run one to watch it move through source, build, and package." />
    </div>
  );
}

function JobList({ query, workspaceSlug, appId, kind, empty }: {
  query: { data?: Simulation[]; isPending: boolean; isError: boolean; isFetching: boolean; error: Error | null; refetch: () => unknown };
  workspaceSlug: string; appId: string; kind: "build" | "simulation"; empty: string;
}) {
  const base = `/workspace/${workspaceSlug}/apps/${appId}/deployments`;
  const Icon = kind === "build" ? Hammer : FlaskConical;
  const label = kind === "build" ? "Build" : "Run";
  if (query.isPending) return <p className="dash-note" role="status">Loading…</p>;
  if (query.isError) {
    return <div className="dash-callout dash-callout--error" role="alert"><p>{query.error?.message}</p><button className="lp-btn lp-btn--light lp-btn--small" type="button" onClick={() => void query.refetch()}>Try again</button></div>;
  }
  if (!query.data?.length) return <p className="dash-note">{empty}</p>;
  return (
    <>
      <ul className="dash-jobs">
        {query.data.map((job, index) => {
          const done = job.stages.filter((stage) => stage.status === "succeeded").length;
          return (
            <li key={job.id} style={{ "--i": index } as React.CSSProperties}>
              <Link className="dash-job" data-status={job.status} href={`${base}/${job.id}${kind === "build" ? "?kind=build" : ""}`}>
                <span className="dash-job-icon"><Icon size={15} aria-hidden="true" /></span>
                <span className="dash-job-main">
                  <strong>{label} <code>{job.id.slice(0, 8)}</code></strong>
                  <span className="dash-job-progress" aria-label={`${done} of ${job.stages.length} steps complete`}>
                    <i style={{ "--p": job.stages.length ? done / job.stages.length : 0 } as React.CSSProperties} />
                  </span>
                </span>
                <StatusPill status={job.status} />
                <span className="dash-job-time">{new Date(job.createdAt).toLocaleString()}</span>
                <ArrowRight size={14} aria-hidden="true" />
              </Link>
            </li>
          );
        })}
      </ul>
      <p className="dash-note"><button className="dash-text-button" type="button" disabled={query.isFetching} onClick={() => void query.refetch()}>{query.isFetching ? "Refreshing…" : "Refresh"}</button></p>
    </>
  );
}
