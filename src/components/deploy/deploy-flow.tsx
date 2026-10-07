"use client";

import { Archive, ArrowRight, Check, GitBranch, Play, TerminalSquare } from "lucide-react";
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
  "No Dockerfile, node_modules, .env files, or credentials in the upload",
];

const sources = [
  { label: "Upload an archive", detail: "A plain .tar of your application files", icon: Archive },
  { label: "sprout deploy", detail: "From your terminal or a coding agent", icon: TerminalSquare },
  { label: "Connect a repository", detail: "Deploy from a branch", icon: GitBranch },
];

function slugify(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 63).replace(/-+$/, "");
}

export function DeployFlow({ workspaceSlug }: { workspaceSlug: string }) {
  const [application, setApplication] = useState<Application | null>(null);

  return (
    <main className="dashboard-page compact-page deploy-page">
      <header className="page-heading">
        <p className="eyebrow">Deploy</p>
        <h1>From code to a healthy URL.</h1>
        <p>Sprout builds your application in a sealed, resource-limited container, checks its health, and only then gives it an address your team can open.</p>
      </header>

      <ol className="deploy-steps">
        <li className={`deploy-step ${application ? "is-done" : "is-current"}`}>
          <StepHeading number={1} title="Name the application" done={!!application} />
          {application
            ? <p className="deploy-summary"><strong>{application.name}</strong> · {application.slug}</p>
            : <CreateApplicationForm workspaceSlug={workspaceSlug} onCreated={setApplication} />}
        </li>

        <li className={`deploy-step ${application ? "is-current" : ""}`}>
          <StepHeading number={2} title="Bring the code" />
          <div className="deploy-sources" role="list">
            {sources.map(({ label, detail, icon: Icon }) => (
              <div className="deploy-source" role="listitem" key={label}>
                <Icon size={17} aria-hidden="true" />
                <div><strong>{label}</strong><span>{detail}</span></div>
                <small>Not available yet</small>
              </div>
            ))}
          </div>
          <details className="deploy-contract">
            <summary>What Sprout accepts today</summary>
            <ul>{contract.map((rule) => <li key={rule}>{rule}</li>)}</ul>
          </details>
        </li>

        <li className={`deploy-step ${application ? "is-current" : ""}`}>
          <StepHeading number={3} title="Run the deployment" />
          {application
            ? <RunDeployment workspaceSlug={workspaceSlug} application={application} />
            : <p className="deploy-muted">Name the application first.</p>}
        </li>
      </ol>

      <RecentApplications workspaceSlug={workspaceSlug} />
    </main>
  );
}

function StepHeading({ number, title, done = false }: { number: number; title: string; done?: boolean }) {
  return (
    <h2 className="deploy-step-heading">
      <span aria-hidden="true">{done ? <Check size={14} /> : number}</span>
      {title}
      {done && <span className="sr-only"> (complete)</span>}
    </h2>
  );
}

function CreateApplicationForm({ workspaceSlug, onCreated }: { workspaceSlug: string; onCreated: (app: Application) => void }) {
  const create = useCreateApplication(workspaceSlug);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [slugEdited, setSlugEdited] = useState(false);
  const effectiveSlug = slugEdited ? slug : slugify(name);

  function submit(event: FormEvent) {
    event.preventDefault();
    create.mutate({ name: name.trim(), slug: effectiveSlug }, { onSuccess: onCreated });
  }

  return (
    <form className="deploy-form" onSubmit={submit}>
      <label>
        Application name
        <input value={name} onChange={(event) => setName(event.target.value)} required maxLength={120}
          placeholder="Invoice approvals" disabled={create.isPending} />
      </label>
      <label>
        URL name
        <input value={effectiveSlug} required maxLength={63} pattern="[a-z0-9]+(-[a-z0-9]+)*"
          title="Lowercase letters, numbers, and single hyphens"
          onChange={(event) => { setSlugEdited(true); setSlug(event.target.value); }} disabled={create.isPending} />
      </label>
      <button className="button button-primary" disabled={create.isPending || !name.trim() || !effectiveSlug}>
        {create.isPending ? "Creating…" : "Create application"}
      </button>
      {create.isError && <p className="deploy-error" role="alert">{create.error.message}</p>}
    </form>
  );
}

function RunDeployment({ workspaceSlug, application }: { workspaceSlug: string; application: Application }) {
  const action = useSimulationActions(workspaceSlug, application.id);
  const router = useRouter();
  const base = `/workspace/${workspaceSlug}/apps/${application.id}/deployments`;

  return (
    <div className="deploy-run">
      <p className="deploy-muted">
        Until code upload is available, this runs a <strong>deployment simulation</strong>: the real worker moves through
        source, build, and package steps, but no application code runs and no live URL is created.
      </p>
      <button className="button button-primary" type="button" disabled={action.isPending}
        onClick={() => action.mutate(undefined, { onSuccess: (job) => router.push(`${base}/${job.id}`) })}>
        <Play size={14} /> {action.isPending ? "Starting…" : "Run simulation"}
      </button>
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
      <h2 id="deploy-recent-heading">Deploy an existing application</h2>
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
