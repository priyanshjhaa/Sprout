"use client";

import { AlertTriangle, Archive, ChevronRight, Globe, Hammer, Moon, Plus, Sprout } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { DropToDeploy } from "@/components/dashboard/drop-to-deploy";
import { WorkspaceHero } from "@/components/dashboard/workspace-hero";
import { Plant } from "@/components/plant";
import { useApplications, useWorkspace } from "@/lib/query/hooks";
import type { AppStatus, Application } from "@/types/domain";

// The lifecycle Sprout keeps visible, in the order an app moves through it.
const phases: Array<{ status: AppStatus; label: string; hint: string; icon: typeof Globe }> = [
  { status: "running", label: "Live", hint: "Serving a public URL", icon: Globe },
  { status: "building", label: "Building", hint: "Being built sealed off", icon: Hammer },
  { status: "failed", label: "Needs attention", hint: "A deployment failed", icon: AlertTriangle },
  { status: "undeployed", label: "Ready", hint: "Created, not deployed", icon: Sprout },
  { status: "paused", label: "Paused", hint: "On hold", icon: Moon },
  { status: "archived", label: "Archived", hint: "Kept for reference", icon: Archive },
];

const labelFor = (status: AppStatus) => phases.find((phase) => phase.status === status)?.label ?? status;

// Apps that need a person come first, then work in motion, then the calm ones.
const attentionOrder: AppStatus[] = ["failed", "building", "undeployed", "running", "paused", "archived"];

type Filter = "all" | AppStatus;

export function WorkspaceHome({ workspaceSlug }: { workspaceSlug: string }) {
  const { data: workspace } = useWorkspace(workspaceSlug);
  // isPending, not isLoading: the query is disabled until Clerk restores the session,
  // and isLoading would read false (and show "no apps") during that gap.
  const { data: apps, isPending: isLoading, isError, refetch } = useApplications(workspaceSlug);
  const [chosenFilter, setFilter] = useState<Filter>("all");
  const base = `/workspace/${workspaceSlug}`;
  const count = (status: AppStatus) => apps?.filter((app) => app.status === status).length ?? 0;
  const sorted = [...(apps ?? [])].sort((a, b) => attentionOrder.indexOf(a.status) - attentionOrder.indexOf(b.status));
  // A state that has run out of apps (the last failure recovered) falls back to all apps.
  const filter: Filter = chosenFilter !== "all" && count(chosenFilter) === 0 ? "all" : chosenFilter;
  const shown = filter === "all" ? sorted : sorted.filter((app) => app.status === filter);
  const tabs: Array<{ id: Filter; label: string }> = [
    { id: "all", label: "All apps" },
    ...phases.filter((phase) => count(phase.status) > 0).map((phase) => ({ id: phase.status, label: phase.label })),
  ];

  return (
    <main className="dashboard-page dash-home">
      <DropToDeploy workspaceSlug={workspaceSlug} />
      <WorkspaceHero
        base={base}
        workspaceName={workspace?.name ?? "Your workspace"}
        appCount={apps?.length ?? 0}
        liveCount={count("running")}
        isLoading={isLoading}
      />

      {!isLoading && !isError && sorted.length > 0 && (
        <div className="dash-tabs" role="tablist" aria-label="Filter apps by state">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              type="button"
              role="tab"
              aria-selected={filter === tab.id}
              onClick={() => setFilter(tab.id)}
            >
              {tab.label}
              <em>{tab.id === "all" ? sorted.length : count(tab.id)}</em>
            </button>
          ))}
        </div>
      )}

      <div className="dash-home-grid">
        <section className="dash-section" aria-labelledby="dash-apps-title">
          <header className="dash-section-head">
            <h2 id="dash-apps-title">
              <Sprout size={17} aria-hidden="true" /> Your apps
            </h2>
            <span>Needs attention first</span>
          </header>

          {isLoading ? (
            <ul className="dash-cards" aria-label="Loading applications">
              {[0, 1, 2].map((item) => (
                <li key={item} className="dash-card dash-card-skeleton" />
              ))}
            </ul>
          ) : isError ? (
            <div className="dash-empty">
              <h3>Applications could not be loaded</h3>
              <p>You can safely try again.</p>
              <button className="lp-btn lp-btn--light lp-btn--small" type="button" onClick={() => refetch()}>
                Try again
              </button>
            </div>
          ) : sorted.length === 0 ? (
            <div className="dash-empty">
              <Plant status="undeployed" />
              <h3>The soil is ready</h3>
              <p>Drop a folder anywhere on this page, or start from the Deploy page.</p>
              <Link className="lp-btn lp-btn--dark lp-btn--small" href={`${base}/deploy`}>
                <Plus size={15} aria-hidden="true" /> Deploy an app
              </Link>
            </div>
          ) : (
            <ul className="dash-cards">
              {shown.map((app, index) => (
                <AppCard key={app.id} app={app} base={base} index={index} />
              ))}
              <li>
                <Link className="dash-card dash-card-new" href={`${base}/deploy`}>
                  <Plus size={20} aria-hidden="true" />
                  <span>Deploy another app</span>
                </Link>
              </li>
            </ul>
          )}
        </section>

        <aside className="dash-section" aria-labelledby="dash-lifecycle-title">
          <header className="dash-section-head">
            <h2 id="dash-lifecycle-title">Lifecycle</h2>
          </header>
          <ul className="dash-lifecycle">
            {phases.map((phase, index) => {
              const Icon = phase.icon;
              const total = count(phase.status);
              return (
                <li key={phase.status} style={{ "--i": index } as React.CSSProperties}>
                  <button
                    type="button"
                    data-status={phase.status}
                    aria-pressed={filter === phase.status}
                    disabled={isLoading || total === 0}
                    onClick={() => setFilter(filter === phase.status ? "all" : phase.status)}
                  >
                    <span className="dash-lifecycle-icon">
                      <Icon size={15} aria-hidden="true" />
                    </span>
                    <span className="dash-lifecycle-text">
                      <b>{phase.label}</b>
                      <small>
                        {isLoading ? "–" : total} {total === 1 ? "app" : "apps"} · {phase.hint}
                      </small>
                    </span>
                    <ChevronRight size={14} aria-hidden="true" />
                  </button>
                </li>
              );
            })}
          </ul>
        </aside>
      </div>
    </main>
  );
}

function AppCard({ app, base, index }: { app: Application; base: string; index: number }) {
  return (
    <li style={{ "--i": index } as React.CSSProperties}>
      <Link className="dash-card" href={`${base}/apps/${app.id}`} data-status={app.status}>
        <span className="dash-card-art" aria-hidden="true">
          <Plant status={app.status} />
        </span>
        <span className="dash-card-body">
          <b>{app.name}</b>
          <small className="dash-card-url">{app.url}</small>
          {app.description && <span className="dash-card-description">{app.description}</span>}
        </span>
        <span className="dash-card-foot">
          <span className="dash-card-status">
            <i data-status={app.status} aria-hidden="true" /> {labelFor(app.status)}
          </span>
          <span className="lp-mono">{app.updatedAt}</span>
        </span>
      </Link>
    </li>
  );
}
