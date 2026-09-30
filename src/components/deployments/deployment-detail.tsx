"use client";

import { Check, Circle, FlaskConical } from "lucide-react";
import Link from "next/link";
import { StatusPill } from "@/components/status-pill";
import { isActiveSimulation } from "@/lib/api/simulation-stream";
import { useSimulation, useSimulationActions } from "@/lib/query/simulations";

export function DeploymentDetail({ workspaceSlug, appId, deploymentId }: { workspaceSlug: string; appId: string; deploymentId: string }) {
  const query = useSimulation(workspaceSlug, appId, deploymentId);
  const action = useSimulationActions(workspaceSlug, appId);
  const job = query.data;
  const base = `/workspace/${workspaceSlug}/apps/${appId}/deployments`;
  return (
    <section className="app-section deployment-detail simulation-view">
      <Link className="section-back" href={base}>← Simulations</Link>
      {query.isPending && <p className="simulation-notice" role="status">Loading simulation…</p>}
      {(query.isError || query.streamError) ? <div className="simulation-notice" role="alert">
        <h2>Progress unavailable</h2><p>{query.streamError ?? query.error?.message}</p>
        <button className="button button-secondary" disabled={query.isFetching} onClick={() => void query.reconnect()}>Retry connection</button>
      </div> : job && <>
        <header className="section-heading">
          <div><div className="deployment-title"><h2>Simulation {job.id.slice(0, 8)}</h2><StatusPill status={job.status} /></div><p>{new Date(job.createdAt).toLocaleString()}</p></div>
          {isActiveSimulation(job) && <button className="button button-secondary" type="button" disabled={action.isPending} onClick={() => action.mutate(job.id)}>{action.isPending ? "Cancelling…" : "Cancel simulation"}</button>}
        </header>
        <p className="simulation-notice"><FlaskConical size={16} /> Simulated work only. No repository is cloned, no container is built, and no app is deployed.</p>
        {action.isError && <p className="simulation-notice" role="alert">{action.error.message}</p>}
        <p className="simulation-note" role="status" aria-live="polite">{isActiveSimulation(job) ? query.streamState === "live" ? "Receiving live progress" : query.streamState === "reconnecting" ? "Reconnecting — the worker continues in the background" : "Connecting to live progress…" : `Simulation ${job.status}.`}</p>
        <ol className="deployment-stages simulation-stages" aria-label="Simulation progress">
          {job.stages.map((stage) => <li className={stage.status === "succeeded" ? "complete" : stage.status === "failed" ? "failed" : ""} key={stage.name}>
            <span>{stage.status === "succeeded" ? <Check size={12} /> : <Circle size={10} />}</span><strong>{stage.name}</strong><small>{stage.status}</small>
          </li>)}
        </ol>
        {job.failureCode && <p className="simulation-notice">Outcome code: <code>{job.failureCode}</code></p>}
        <p className="simulation-note">Closing this page stops listening, not the simulation. Build logs will appear here when real, isolated builds are implemented.</p>
      </>}
    </section>
  );
}
