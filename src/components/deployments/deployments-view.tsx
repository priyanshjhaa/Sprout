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
    <section className="app-section simulation-view">
      <header className="section-heading">
        <div>
          <h2>Deployments</h2>
          <p>{localBuilds
            ? "Builds run in a sealed container on this machine. Nothing is started or given a URL yet."
            : "Runs are simulated for now: no code executes and no URL goes live."}</p>
        </div>
      </header>
      {application.data && !active && <p className="simulation-note">Resume this application in Settings before deploying.</p>}
      {localBuilds && <Builds workspaceSlug={workspaceSlug} appId={appId} active={active} />}
      <Simulations workspaceSlug={workspaceSlug} appId={appId} active={active} titled={localBuilds} />
    </section>
  );
}

function Builds({ workspaceSlug, appId, active }: { workspaceSlug: string; appId: string; active: boolean }) {
  const query = useBuilds(workspaceSlug, appId, true);
  const busy = query.data?.some(isActiveSimulation);
  return (
    <div className="deployment-group">
      <div className="deployment-group-heading">
        <h3>Builds</h3>
        <UploadBuild workspaceSlug={workspaceSlug} appId={appId} disabled={!active || busy} />
      </div>
      <JobList query={query} workspaceSlug={workspaceSlug} appId={appId} kind="build"
        empty="Upload a .tar of your app to build it." />
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
    <div className="deployment-group">
      <div className="deployment-group-heading">
        {titled ? <h3>Simulations</h3> : <span />}
        <button className="button button-secondary" type="button" disabled={!canStart}
          onClick={() => action.mutate(undefined, { onSuccess: (job) => router.push(`${base}/${job.id}`) })}>
          <Play size={14} /> {action.isPending ? "Starting…" : "Run simulation"}
        </button>
      </div>
      {action.isError && <p className="simulation-notice" role="alert">{action.error.message} Check the list before trying again.</p>}
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
  if (query.isPending) return <p className="simulation-notice" role="status">Loading…</p>;
  if (query.isError) {
    return <div className="simulation-notice" role="alert"><p>{query.error?.message}</p><button className="button button-secondary" onClick={() => void query.refetch()}>Try again</button></div>;
  }
  if (!query.data?.length) return <p className="simulation-note">{empty}</p>;
  return (
    <>
      <div className="deployment-list">
        {query.data.map((job) => (
          <Link className="deployment-row" href={`${base}/${job.id}${kind === "build" ? "?kind=build" : ""}`} key={job.id}>
            <span className="deployment-icon"><Icon size={15} /></span>
            <div className="deployment-main"><div><strong>{label} {job.id.slice(0, 8)}</strong></div><span>{job.stages.filter((stage) => stage.status === "succeeded").length} of 3 steps complete</span></div>
            <StatusPill status={job.status} />
            <div className="deployment-time"><strong>{new Date(job.createdAt).toLocaleString()}</strong></div>
            <ArrowRight className="deployment-arrow" size={14} />
          </Link>
        ))}
      </div>
      <p className="simulation-note"><button className="simulation-refresh" type="button" disabled={query.isFetching} onClick={() => void query.refetch()}>Refresh</button></p>
    </>
  );
}
