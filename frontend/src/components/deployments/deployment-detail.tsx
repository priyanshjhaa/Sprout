"use client";

import { ArrowLeft, Check, ExternalLink, FlaskConical, Hammer } from "lucide-react";
import Link from "next/link";
import { Pipeline } from "@/components/pipeline";
import { StatusPill } from "@/components/status-pill";
import { isActiveSimulation } from "@/lib/api/simulation-stream";
import type { JobKind } from "@/lib/api/simulations";
import { useApplication } from "@/lib/query/hooks";
import { useBuildActions, useSimulation, useSimulationActions } from "@/lib/query/simulations";

// Plain-language explanations for the stable failure codes a build can end with.
const failureExplanations: Record<string, string> = {
  node_contract_invalid: "The folder doesn't follow the Node.js app rules: check package.json, the lockfile, and the build script.",
  node_build_dependencies_unsupported: "This Sprout server can't install npm packages.",
  node_build_install_failed: "Installing the app's npm packages failed. Packages that need an install script to work (native modules such as bcrypt, sharp or sqlite3) aren't supported yet.",
  npm_registry_unavailable: "Sprout couldn't download the app's packages from the npm registry. Try again in a minute.",
  npm_package_not_found: "A package in package-lock.json isn't on the public npm registry. Private packages aren't supported yet.",
  npm_package_integrity_mismatch: "A downloaded package didn't match the checksum in package-lock.json, so Sprout didn't use it.",
  npm_packages_too_large: "The app's packages are larger than Sprout's limit (160 MiB in all).",
  npm_package_cache_unavailable: "Sprout couldn't use its package cache. Check the server's disk.",
  node_build_failed: "The app's build script failed.",
  node_build_artifact_invalid: "The build didn't produce a usable dist/ folder.",
  node_build_artifact_too_large: "The build output, including the packages the app needs to run, is larger than the limit (16 MiB).",
  node_build_docker_unavailable: "Docker or the pinned Node image isn't available on this machine.",
  source_archive_invalid: "The upload wasn't a readable archive. Drop the folder again; if you chose a .tar file, it must be a plain, uncompressed one.",
  source_limit_exceeded: "The folder has too many files or is too large (up to 256 files and 8 MiB).",
  source_sensitive_path: "The folder contains a sensitive file such as a key or .env. Dropping the folder leaves those out for you; if you used a .tar, remove it and upload again.",
  source_unavailable: "The upload was no longer available. Deploy again.",
  process_interrupted: "The server restarted while this was running.",
  job_timeout: "It took longer than the time limit.",
  node_contract_engine: "package.json needs \"engines\": { \"node\": \"24.x\" } so Sprout knows which Node.js version to use.",
  node_contract_private: "package.json needs \"private\": true.",
  node_contract_scripts: "package.json needs a \"build\" script. Add a \"start\" script to run a server; without one, the built dist/ folder is served as a website.",
  node_contract_manifest: "package.json is missing, unreadable, or has no name and version.",
  node_contract_lockfile: "package-lock.json is missing or doesn't match package.json. Run \"npm install\" and include the file.",
  node_contract_workspaces: "Workspaces (monorepos) aren't supported yet.",
  node_contract_files: "The folder contains files Sprout doesn't accept in a Node.js app (such as a Dockerfile or a symlink).",
  static_contract_invalid: "It is not a Node.js app, a Python app or a valid static site. A static site needs index.html at the root, no package.json or requirements.txt, and plain file names (letters, numbers, dots, dashes, underscores; no dotfiles). A website built with npm needs the same in its dist/ folder.",
  python_contract_requirements: "requirements.txt must pin every package with == and list its hashes, with no other options. Create it with \"uv pip compile requirements.in --generate-hashes -o requirements.txt\" (or \"uv export\", or \"pip-compile --generate-hashes\").",
  python_contract_procfile: "A Python app needs a Procfile with exactly one line, \"web: <command>\", for example \"web: gunicorn app:app --bind 0.0.0.0:$PORT\".",
  python_contract_files: "The folder contains files Sprout doesn't accept in a Python app: a virtual environment (pyvenv.cfg), a site-packages folder, or a Dockerfile.",
  python_build_dependencies_unsupported: "This Sprout server can't install Python packages.",
  python_build_install_failed: "Installing the app's Python packages failed. Every package must be in requirements.txt with its hashes, and have a wheel for Linux and Python 3.13.",
  python_build_failed: "A .py file has a syntax error, or isn't valid Python 3.13.",
  python_build_artifact_invalid: "The build didn't produce a usable app.",
  python_build_artifact_too_large: "The app with its installed packages is larger than the limit (16 MiB).",
  python_build_docker_unavailable: "Docker or the pinned Python image isn't available on this machine.",
  python_registry_unavailable: "Sprout couldn't download the app's packages from PyPI. Try again in a minute.",
  python_package_not_found: "A package version in requirements.txt isn't on PyPI. Private packages aren't supported yet.",
  python_package_no_wheel: "A package in requirements.txt has no ready-made wheel for Linux and Python 3.13. Sprout doesn't build packages from source, because that runs the package's own setup code. Try a newer version of the package.",
  python_package_integrity_mismatch: "A downloaded package didn't match the hash in requirements.txt, so Sprout didn't use it.",
  python_packages_too_large: "The app's packages are larger than Sprout's limit (160 MiB in all, 64 MiB for one).",
  python_package_cache_unavailable: "Sprout couldn't use its package cache. Check the server's disk.",
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
  const live = job && isActiveSimulation(job);
  return (
    <section className="dash-stack">
      <Link className="dash-back" href={base}><ArrowLeft size={14} aria-hidden="true" /> Deployments</Link>
      {query.isPending && <p className="dash-note" role="status">Loading deployment…</p>}
      {(query.isError || query.streamError) ? <div className="dash-callout dash-callout--error" role="alert">
        <h2>Progress unavailable</h2><p>{query.streamError ?? query.error?.message}</p>
        <button className="lp-btn lp-btn--light lp-btn--small" type="button" disabled={query.isFetching} onClick={() => void query.reconnect()}>Retry connection</button>
      </div> : job && <>
        <header className="dash-subhead">
          <div>
            <div className="dash-subhead-title"><h2>{isBuild ? "Build" : "Run"} <code>{job.id.slice(0, 8)}</code></h2><StatusPill status={job.status} /></div>
            <p>{new Date(job.createdAt).toLocaleString()}</p>
          </div>
          {live && <button className="lp-btn lp-btn--light lp-btn--small" type="button" disabled={action.isPending} onClick={() => action.cancel(job.id)}>{action.isPending ? "Cancelling…" : "Cancel"}</button>}
        </header>
        {action.isError && <p className="dash-callout dash-callout--error" role="alert">{action.error?.message}</p>}
        <p className="dash-stream" data-state={live ? query.streamState : "done"} role="status" aria-live="polite">
          <i aria-hidden="true" />
          {live ? query.streamState === "live" ? "Receiving live progress" : query.streamState === "reconnecting" ? "Reconnecting — the worker continues in the background" : "Connecting to live progress…" : job.status === "live" ? "This deployment is live." : `${isBuild ? "Build" : "Run"} ${job.status}.`}
        </p>
        <div className={`dash-pipeline-card sp-run sp-run-${job.status}`}><Pipeline stages={job.stages} simulated={!isBuild} /></div>
        {job.failureCode && <div className="dash-callout dash-callout--error"><p>{failureExplanations[job.failureCode] ?? "It did not finish."}</p><code>{job.failureCode}</code></div>}
        {isBuild && job.status === "live" && <div className="dash-callout dash-callout--ok"><p><Check className="sp-inline-icon" size={14} aria-hidden="true" /> Live{application?.liveUrl
          ? <> at <a href={application.liveUrl} target="_blank" rel="noreferrer">{application.liveUrl}<ExternalLink className="sp-inline-icon sp-inline-icon-after" size={12} aria-hidden="true" /></a>.</>
          : "."} Anyone with the link can open it.</p></div>}
        {isBuild && job.status === "succeeded" && <div className="dash-callout dash-callout--ok"><p><Check className="sp-inline-icon" size={14} aria-hidden="true" /> Built and stored. A newer version is live, or this one has not been started.</p></div>}
        <p className="dash-note">{isBuild
          ? <><Hammer className="sp-inline-icon" size={13} aria-hidden="true" /> Built in a sealed container on this machine. Leaving this page does not stop it.</>
          : <><FlaskConical className="sp-inline-icon" size={13} aria-hidden="true" /> Simulated run: no code executes. Leaving this page does not stop it.</>}</p>
      </>}
    </section>
  );
}
