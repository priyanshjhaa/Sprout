"use client";

import { ArrowRight, Check, Play, Rocket } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { FormEvent, useState } from "react";
import { Pipeline } from "@/components/pipeline";
import { UploadBuild } from "@/components/deployments/upload-build";
import { slugify } from "@/lib/slug";
import { useApplications, useCreateApplication } from "@/lib/query/hooks";
import { useCapabilities, useSimulationActions } from "@/lib/query/simulations";
import type { Application } from "@/types/domain";
import { PageHeader } from "@/components/dashboard/page-header";

// The planned front doors, all calling the same deploy API (deployment-strategy.md at the repository root).
const frontDoors = [
  "Drop or choose your site's folder in this dashboard (when local builds are on). A ready-made .tar works too",
  "sprout deploy from your terminal (coming soon)",
  "A deploy tool for coding agents (MCP) that takes a folder or a GitHub repository (coming soon)",
];

// Mirrors docs/node-build-contract.md and docs/static-sites-learning.md.
const nodeContract = [
  "One npm application with package.json and package-lock.json at the root",
  "Node.js 24 (engines.node: \"24.x\") with build and start scripts",
  "Builds into a dist/ folder; serves /health on PORT",
  "No Dockerfile, node_modules, .env files, or credentials",
];
const staticContract = [
  "A folder with an index.html at the top and no package.json (hidden files are left out for you)",
  "Plain file names: letters, numbers, dots, dashes and underscores, no dotfiles",
  "At most 256 files and 8 MiB; an optional 404.html is used for missing pages",
  "Served as-is, with nothing built or executed. Python apps are next",
];

export function DeployFlow({ workspaceSlug }: { workspaceSlug: string }) {
  const [application, setApplication] = useState<Application | null>(null);
  const localBuilds = useCapabilities().data?.localBuilds === true;

  return (
    <main className="dashboard-page compact-page dash-stack">
      <header className="dash-deploy-head">
        <PageHeader
          eyebrow="Deploy"
          title="From code to a public URL"
          description="Sprout builds it sealed off, checks its health, and only then gives it an address."
        />
        <Pipeline stages={application ? [{ name: "source", status: "running" }] : undefined} />
      </header>

      <section className="dash-panel-card dash-deploy-card" aria-live="polite">
        <p className="dash-step"><b>{application ? 2 : 1}</b> {application ? "Give Sprout your site" : "Name your application"}</p>
        {application
          ? <RunDeployment workspaceSlug={workspaceSlug} application={application} localBuilds={localBuilds} onReset={() => setApplication(null)} />
          : <CreateApplicationForm workspaceSlug={workspaceSlug} onCreated={setApplication} />}
      </section>

      <div className="dash-guides">
        {localBuilds
          ? <p>Drop your site&rsquo;s folder. Sprout packs it for you, builds it in a sealed container on this machine (a plain website is stored as it is), checks it, and gives it a public URL.</p>
          : <p>Until uploads are enabled, a run is a simulation: no code executes.</p>}
        <details>
          <summary>Ways to deploy</summary>
          <ul>{frontDoors.map((door) => <li key={door}>{door}</li>)}</ul>
          <p>The CLI and coding agents sign in with an <Link href={`/workspace/${workspaceSlug}/tokens`}>access token</Link>.</p>
        </details>
        <details>
          <summary>Node.js apps</summary>
          <ul>{nodeContract.map((rule) => <li key={rule}>{rule}</li>)}</ul>
        </details>
        <details>
          <summary>Static sites</summary>
          <ul>{staticContract.map((rule) => <li key={rule}>{rule}</li>)}</ul>
        </details>
      </div>

      <RecentApplications workspaceSlug={workspaceSlug} />
    </main>
  );
}

function CreateApplicationForm({ workspaceSlug, onCreated }: { workspaceSlug: string; onCreated: (app: Application) => void }) {
  const create = useCreateApplication(workspaceSlug);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [editingSlug, setEditingSlug] = useState(false);
  const effectiveSlug = editingSlug ? slug : slugify(name);

  function submit(event: FormEvent) {
    event.preventDefault();
    create.mutate({ name: name.trim(), slug: effectiveSlug }, { onSuccess: onCreated });
  }

  return (
    <form className="dash-deploy-form" onSubmit={submit}>
      <label className="dash-deploy-name">
        Application name
        <input value={name} onChange={(event) => setName(event.target.value)} required maxLength={120}
          placeholder="Invoice approvals" disabled={create.isPending} autoFocus />
      </label>
      <button className="lp-btn lp-btn--dark" disabled={create.isPending || !name.trim() || !effectiveSlug}>
        {create.isPending ? "Creating…" : "Create application"}
      </button>
      <div className="dash-deploy-slug">
        {editingSlug ? (
          <label>
            <span className="sr-only">URL name</span>
            <input value={slug} onChange={(event) => setSlug(event.target.value)} required maxLength={63}
              pattern="[a-z0-9]+(-[a-z0-9]+)*" title="Lowercase letters, numbers, and single hyphens" disabled={create.isPending} />
          </label>
        ) : (
          <>
            <span>URL name: <strong>{effectiveSlug || "set from the name"}</strong></span>
            <button type="button" className="dash-text-button" onClick={() => { setSlug(effectiveSlug); setEditingSlug(true); }}>Edit</button>
          </>
        )}
      </div>
      {create.isError && <p className="dash-callout dash-callout--error" role="alert">{create.error.message}</p>}
    </form>
  );
}

function RunDeployment({ workspaceSlug, application, localBuilds, onReset }: { workspaceSlug: string; application: Application; localBuilds: boolean; onReset: () => void }) {
  const action = useSimulationActions(workspaceSlug, application.id);
  const router = useRouter();
  const base = `/workspace/${workspaceSlug}/apps/${application.id}/deployments`;
  const simulate = () => action.mutate(undefined, { onSuccess: (job) => router.push(`${base}/${job.id}`) });

  return (
    <div className="dash-deploy-run">
      <p className="dash-callout dash-callout--ok"><Check size={15} aria-hidden="true" /> <strong>{application.name}</strong> is ready to deploy.</p>
      {localBuilds && <UploadBuild workspaceSlug={workspaceSlug} appId={application.id} />}
      <div className="dash-deploy-run-actions">
        {localBuilds
          ? <button className="dash-text-button" type="button" disabled={action.isPending} onClick={simulate}>{action.isPending ? "Starting…" : "Run a simulation instead"}</button>
          : <button className="lp-btn lp-btn--dark lp-btn--small" type="button" disabled={action.isPending} onClick={simulate}>
              <Play size={14} /> {action.isPending ? "Starting…" : "Run simulation"}
            </button>}
        <button className="dash-text-button" type="button" onClick={onReset}>Create another</button>
      </div>
      {action.isError && (
        <p className="dash-callout dash-callout--error" role="alert">
          {action.error.message} <Link href={base}>Check its deployments</Link> before trying again.
        </p>
      )}
    </div>
  );
}

function RecentApplications({ workspaceSlug }: { workspaceSlug: string }) {
  const { data } = useApplications(workspaceSlug);
  const recent = data?.slice(0, 4) ?? [];
  if (recent.length === 0) return null;

  return (
    <section className="dash-stack" aria-labelledby="deploy-recent-heading">
      <h2 id="deploy-recent-heading" className="dash-section-title">Or deploy an existing app</h2>
      <ul className="dash-link-list">
        {recent.map((app, index) => (
          <li key={app.id} style={{ "--i": index } as React.CSSProperties}>
            <Link href={`/workspace/${workspaceSlug}/apps/${app.id}/deployments`}>
              <span className="dash-next-icon"><Rocket size={15} aria-hidden="true" /></span>
              <span><strong>{app.name}</strong><small>{app.url}</small></span>
              <ArrowRight size={14} aria-hidden="true" />
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}
