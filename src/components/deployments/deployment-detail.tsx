"use client";

import { Check, ExternalLink, FlaskConical, Hammer } from "lucide-react";
import Link from "next/link";
import { Pipeline } from "@/components/pipeline";
import { StatusPill } from "@/components/status-pill";
import { isActiveSimulation } from "@/lib/api/simulation-stream";
import type { JobKind } from "@/lib/api/simulations";
import { useApplication } from "@/lib/query/hooks";
import { useBuildActions, useSimulation, useSimulationActions } from "@/lib/query/simulations";

// Plain-language explanations for the stable failure codes a build can end with.
const failureExplanations: Record<string, string> = {
  node_contract_invalid: "The folder doesn't follow the Node.js app rules: check package.json, the lockfile, and the build and start scripts.",
  node_build_dependencies_unsupported: "This app uses npm packages, and Sprout can't install packages yet. Plain websites, and Node.js apps that use no packages, work today.",
  node_build_failed: "The app's build script failed.",
  node_build_artifact_invalid: "The build didn't produce a usable dist/ folder.",
  node_build_artifact_too_large: "The build output is larger than the limit.",
  node_build_docker_unavailable: "Docker or the pinned Node image isn't available on this machine.",
  source_archive_invalid: "The upload wasn't a readable archive. Drop the folder again; if you chose a .tar file, it must be a plain, uncompressed one.",
  source_limit_exceeded: "The folder has too many files or is too large (up to 256 files and 8 MiB).",
  source_sensitive_path: "The folder contains a sensitive file such as a key or .env. Dropping the folder leaves those out for you; if you used a .tar, remove it and upload again.",
  source_unavailable: "The upload was no longer available. Deploy again.",
  process_interrupted: "The server restarted while this was running.",
  job_timeout: "It took longer than the time limit.",
  node_contract_engine: "package.json needs \"engines\": { \"node\": \"24.x\" } so Sprout knows which Node.js version to use.",
  node_contract_private: "package.json needs \"private\": true.",
  node_contract_scripts: "package.json needs both a \"build\" script and a \"start\" script.",
  node_contract_manifest: "package.json is missing, unreadable, or has no name and version.",
  node_contract_lockfile: "package-lock.json is missing or doesn't match package.json. Run \"npm install\" and include the file.",
  node_contract_workspaces: "Workspaces (monorepos) aren't supported yet.",
  node_contract_files: "The folder contains files Sprout doesn't accept in a Node.js app (such as a Dockerfile or a symlink).",
  static_contract_invalid: "It is neither a Node.js app nor a valid static site. A static site needs index.html at the root, no package.json, and plain file names (letters, numbers, dots, dashes, underscores; no dotfiles).",
  provision_failed: "The built app could not be stored for running.",
  release_failed: "The app could not be made ready to serve.",
  health_check_failed: "The app started but did not answer GET /health with 200 within 30 seconds. Make sure it listens on 0.0.0.0 and the PORT environment variable. Any earlier version is still serving.",
  runtime_unavailable: "Sprout could not reach Docker to run the app. Is Docker running?",
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
  const { data: application } = useApplication(workspaceSlug, appId);
  const base = `/workspace/${workspaceSlug}/apps/${appId}/deployments`;
  return (
    <section className="app-section deployment-detail simulation-view sp-detail">
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
        <p className="simulation-note" role="status" aria-live="polite">{isActiveSimulation(job) ? query.streamState === "live" ? "Receiving live progress" : query.streamState === "reconnecting" ? "Reconnecting — the worker continues in the background" : "Connecting to live progress…" : job.status === "live" ? "This deployment is live." : `${isBuild ? "Build" : "Run"} ${job.status}.`}</p>
        <div className={`sp-run sp-run-${job.status}`}><Pipeline stages={job.stages} simulated={!isBuild} /></div>
        {job.failureCode && <p className="simulation-notice">{failureExplanations[job.failureCode] ?? "It did not finish."} <code>{job.failureCode}</code></p>}
        {/* One paragraph inside the notice: the notice is a grid, so loose inline pieces would each become a row. */}
        {isBuild && job.status === "live" && <div className="simulation-notice sp-notice-flow"><p><Check className="sp-inline-icon" size={14} /> Live{application?.liveUrl
          ? <> at <a href={application.liveUrl} target="_blank" rel="noreferrer">{application.liveUrl}<ExternalLink className="sp-inline-icon sp-inline-icon-after" size={12} aria-hidden="true" /></a>.</>
          : "."} Anyone with the link can open it.</p></div>}
        {isBuild && job.status === "succeeded" && <div className="simulation-notice sp-notice-flow"><p><Check className="sp-inline-icon" size={14} /> Built and stored. A newer version is live, or this one has not been started.</p></div>}
        <p className="simulation-note">{isBuild
          ? <><Hammer className="sp-inline-icon" size={13} /> Built in a sealed container on this machine. Leaving this page does not stop it.</>
          : <><FlaskConical className="sp-inline-icon" size={13} /> Simulated run: no code executes. Leaving this page does not stop it.</>}</p>
      </>}
    </section>
  );
}
