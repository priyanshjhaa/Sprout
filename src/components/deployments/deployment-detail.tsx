"use client";

import { Check, Circle, FlaskConical, Hammer } from "lucide-react";
import Link from "next/link";
import { StatusPill } from "@/components/status-pill";
import { isActiveSimulation } from "@/lib/api/simulation-stream";
import type { JobKind } from "@/lib/api/simulations";
import { useBuildActions, useSimulation, useSimulationActions } from "@/lib/query/simulations";

// Plain-language explanations for the stable failure codes a build can end with.
const failureExplanations: Record<string, string> = {
  node_contract_invalid: "The archive doesn't follow the Node.js app contract: check package.json, the lockfile, and the build and start scripts.",
  node_build_dependencies_unsupported: "Only apps without third-party dependencies can build for now.",
  node_build_failed: "The app's build script failed.",
  node_build_artifact_invalid: "The build didn't produce a usable dist/ folder.",
  node_build_artifact_too_large: "The build output is larger than the limit.",
  node_build_docker_unavailable: "Docker or the pinned Node image isn't available on this machine.",
  source_archive_invalid: "The upload isn't a plain, uncompressed .tar archive.",
  source_limit_exceeded: "The archive has too many files or is too large once extracted.",
  source_sensitive_path: "The archive contains a sensitive file such as .env, .git or a key. Remove it and upload again.",
  source_unavailable: "The uploaded archive was no longer available.",
  process_interrupted: "The server restarted while this was running.",
  job_timeout: "It took longer than the time limit.",
};

export function DeploymentDetail({ workspaceSlug, appId, deploymentId, kind }: { workspaceSlug: string; appId: string; deploymentId: string; kind: JobKind }) {
  const query = useSimulation(workspaceSlug, appId, deploymentId, kind);
  const simulationAction = useSimulationActions(workspaceSlug, appId);
  const buildAction = useBuildActions(workspaceSlug, appId);
  const isBuild = kind === "build";
  const action = isBuild
    ? { isPending: buildAction.isPending, isError: buildAction.isError, error: buildAction.error, cancel: (id: string) => buildAction.mutate({ cancel: id }) }
    : { isPending: simulationAction.isPending, isError: simulationAction.isError, error: simulationAction.error, cancel: (id: string) => simulationAction.mutate(id) };
  const job = query.data;
  const base = `/workspace/${workspaceSlug}/apps/${appId}/deployments`;
  return (
    <section className="app-section deployment-detail simulation-view">
      <Link className="section-back" href={base}>← Deployments</Link>
      {query.isPending && <p className="simulation-notice" role="status">Loading simulation…</p>}
      {(query.isError || query.streamError) ? <div className="simulation-notice" role="alert">
        <h2>Progress unavailable</h2><p>{query.streamError ?? query.error?.message}</p>
        <button className="button button-secondary" disabled={query.isFetching} onClick={() => void query.reconnect()}>Retry connection</button>
      </div> : job && <>
        <header className="section-heading">
          <div><div className="deployment-title"><h2>{isBuild ? "Build" : "Run"} {job.id.slice(0, 8)}</h2><StatusPill status={job.status} /></div><p>{new Date(job.createdAt).toLocaleString()}</p></div>
          {isActiveSimulation(job) && <button className="button button-secondary" type="button" disabled={action.isPending} onClick={() => action.cancel(job.id)}>{action.isPending ? "Cancelling…" : "Cancel"}</button>}
        </header>
        {action.isError && <p className="simulation-notice" role="alert">{action.error?.message}</p>}
        <p className="simulation-note" role="status" aria-live="polite">{isActiveSimulation(job) ? query.streamState === "live" ? "Receiving live progress" : query.streamState === "reconnecting" ? "Reconnecting — the worker continues in the background" : "Connecting to live progress…" : `${isBuild ? "Build" : "Run"} ${job.status}.`}</p>
        <ol className="deployment-stages simulation-stages" aria-label="Simulation progress">
          {job.stages.map((stage) => <li className={stage.status === "succeeded" ? "complete" : stage.status === "failed" ? "failed" : ""} key={stage.name}>
            <span>{stage.status === "succeeded" ? <Check size={12} /> : <Circle size={10} />}</span><strong>{stage.name}</strong><small>{stage.status}</small>
          </li>)}
        </ol>
        {job.failureCode && <p className="simulation-notice">{failureExplanations[job.failureCode] ?? "It did not finish."} <code>{job.failureCode}</code></p>}
        {isBuild && job.status === "succeeded" && <p className="simulation-notice"><Check size={14} /> Built and stored. Starting it and giving it a URL comes next.</p>}
        <p className="simulation-note">{isBuild
          ? <><Hammer size={13} /> Built in a sealed container on this machine. Leaving this page does not stop it.</>
          : <><FlaskConical size={13} /> Simulated run: no code executes. Leaving this page does not stop it.</>}</p>
      </>}
    </section>
  );
}
