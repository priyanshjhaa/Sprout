"use client";

import { ArrowRight, Check, Play } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { FormEvent, useState } from "react";
import { useApplications, useCreateApplication } from "@/lib/query/hooks";
import { useSimulationActions } from "@/lib/query/simulations";
import type { Application } from "@/types/domain";

// Mirrors the Node.js application contract in node-build-contract.md.
const contract = [
  "One npm application with package.json and package-lock.json at the root",
  "Node.js 24 (engines.node: \"24.x\") with build and start scripts",
  "Builds into a dist/ folder; serves /health on PORT",
  "No Dockerfile, node_modules, .env files, or credentials",
];

function slugify(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 63).replace(/-+$/, "");
}

export function DeployFlow({ workspaceSlug }: { workspaceSlug: string }) {
  const [application, setApplication] = useState<Application | null>(null);

  return (
    <main className="dashboard-page compact-page deploy-page">
      <header className="page-heading">
        <h1>Deploy an app</h1>
        <p>Sprout builds it sealed off, checks its health, and only then gives your team an address.</p>
      </header>

      <section className="deploy-card" aria-live="polite">
        {application
          ? <RunDeployment workspaceSlug={workspaceSlug} application={application} onReset={() => setApplication(null)} />
          : <CreateApplicationForm workspaceSlug={workspaceSlug} onCreated={setApplication} />}
      </section>

      <div className="deploy-note">
        <p>Code upload, <code>sprout deploy</code>, and repository deploys are on the way. Until then, a run is a simulation: no code executes.</p>
        <details className="deploy-contract">
          <summary>What Sprout will accept</summary>
          <ul>{contract.map((rule) => <li key={rule}>{rule}</li>)}</ul>
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
    <form className="deploy-form" onSubmit={submit}>
      <label className="deploy-name">
        Application name
        <input value={name} onChange={(event) => setName(event.target.value)} required maxLength={120}
          placeholder="Invoice approvals" disabled={create.isPending} autoFocus />
      </label>
      <button className="button button-primary" disabled={create.isPending || !name.trim() || !effectiveSlug}>
        {create.isPending ? "Creating…" : "Create application"}
      </button>
      <div className="deploy-slug">
        {editingSlug ? (
          <label>
            <span className="sr-only">URL name</span>
            <input value={slug} onChange={(event) => setSlug(event.target.value)} required maxLength={63}
              pattern="[a-z0-9]+(-[a-z0-9]+)*" title="Lowercase letters, numbers, and single hyphens" disabled={create.isPending} />
          </label>
        ) : (
          <>
            <span>URL name: <strong>{effectiveSlug || "set from the name"}</strong></span>
            <button type="button" className="text-button" onClick={() => { setSlug(effectiveSlug); setEditingSlug(true); }}>Edit</button>
          </>
        )}
      </div>
      {create.isError && <p className="deploy-error" role="alert">{create.error.message}</p>}
    </form>
  );
}

function RunDeployment({ workspaceSlug, application, onReset }: { workspaceSlug: string; application: Application; onReset: () => void }) {
  const action = useSimulationActions(workspaceSlug, application.id);
  const router = useRouter();
  const base = `/workspace/${workspaceSlug}/apps/${application.id}/deployments`;

  return (
    <div className="deploy-run">
      <p className="deploy-created"><Check size={15} aria-hidden="true" /> <strong>{application.name}</strong> is ready to deploy.</p>
      <div className="deploy-run-actions">
        <button className="button button-primary" type="button" disabled={action.isPending}
          onClick={() => action.mutate(undefined, { onSuccess: (job) => router.push(`${base}/${job.id}`) })}>
          <Play size={14} /> {action.isPending ? "Starting…" : "Run simulation"}
        </button>
        <button className="text-button" type="button" onClick={onReset}>Create another</button>
      </div>
      {action.isError && (
        <p className="deploy-error" role="alert">
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
    <section className="deploy-recent" aria-labelledby="deploy-recent-heading">
      <h2 id="deploy-recent-heading">Or deploy an existing app</h2>
      <ul>
        {recent.map((app) => (
          <li key={app.id}>
            <Link href={`/workspace/${workspaceSlug}/apps/${app.id}/deployments`}>
              <span>{app.name}</span><ArrowRight size={14} aria-hidden="true" />
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}
