"use client";

import { ArrowRight } from "lucide-react";
import Link from "next/link";
import { useCallback, useRef } from "react";
import { DropToDeploy } from "@/components/dashboard/drop-to-deploy";
import { WorkspaceHero } from "@/components/dashboard/workspace-hero";
import { Plant } from "@/components/plant";
import { StatusPill } from "@/components/status-pill";
import { useApplications, useWorkspace } from "@/lib/query/hooks";
import type { AppStatus, Application } from "@/types/domain";

// The lifecycle Sprout keeps visible, in the order an app moves through it.
const phases: Array<{ status: AppStatus; label: string; hint: string }> = [
  { status: "undeployed", label: "Ready", hint: "created, not deployed" },
  { status: "building", label: "Building", hint: "being built sealed off" },
  { status: "running", label: "Live", hint: "serving a public URL" },
  { status: "failed", label: "Needs attention", hint: "a deployment failed" },
  { status: "paused", label: "Paused", hint: "on hold" },
  { status: "archived", label: "Archived", hint: "kept for reference" },
];

// Apps that need a person come first, then work in motion, then the calm ones.
const attentionOrder: AppStatus[] = ["failed", "building", "undeployed", "running", "paused", "archived"];

// Plants lean a few degrees toward the pointer, like seedlings toward light.
function useLean() {
  const frame = useRef(0);
  const onPointerMove = useCallback((event: React.PointerEvent<HTMLElement>) => {
    if (event.pointerType !== "mouse") return;
    const { currentTarget, clientX } = event;
    cancelAnimationFrame(frame.current);
    frame.current = requestAnimationFrame(() => {
      for (const plant of currentTarget.querySelectorAll<HTMLElement>(".sp-bed-plant")) {
        const box = plant.getBoundingClientRect();
        const offset = (clientX - (box.left + box.width / 2)) / 220;
        plant.style.setProperty("--lean", `${Math.max(-7, Math.min(7, offset * 7)).toFixed(2)}deg`);
      }
    });
  }, []);
  const onPointerLeave = useCallback((event: React.PointerEvent<HTMLElement>) => {
    cancelAnimationFrame(frame.current);
    for (const plant of event.currentTarget.querySelectorAll<HTMLElement>(".sp-bed-plant")) plant.style.setProperty("--lean", "0deg");
  }, []);
  return { onPointerMove, onPointerLeave };
}

export function WorkspaceHome({ workspaceSlug }: { workspaceSlug: string }) {
  const lean = useLean();
  const { data: workspace } = useWorkspace(workspaceSlug);
  // isPending, not isLoading: the query is disabled until Clerk restores the session,
  // and isLoading would read false (and show "no apps") during that gap.
  const { data: apps, isPending: isLoading, isError, refetch } = useApplications(workspaceSlug);
  const base = `/workspace/${workspaceSlug}`;
  const count = (status: AppStatus) => apps?.filter((app) => app.status === status).length ?? 0;
  const sorted = [...(apps ?? [])].sort((a, b) => attentionOrder.indexOf(a.status) - attentionOrder.indexOf(b.status));
  const live = count("running");

  return (
    <main className="dashboard-page sp-home">
      <DropToDeploy workspaceSlug={workspaceSlug} />
      <WorkspaceHero
        base={base}
        workspaceName={workspace?.name ?? "Your workspace"}
        appCount={apps?.length ?? 0}
        liveCount={live}
        isLoading={isLoading}
      />

      <section className="sp-bed" aria-label="Your apps as plants" {...lean}>
        {sorted.length ? (
          <ul className="sp-bed-row">
            {sorted.slice(0, 10).map((app, index) => (
              <li key={app.id} style={{ "--i": index } as React.CSSProperties}>
                <Link href={`${base}/apps/${app.id}`} className={`sp-bed-plant sp-bed-${app.status}`} title={`${app.name}: ${app.status}`}>
                  <Plant status={app.status} />
                  <span className="sp-bed-label"><strong>{app.name}</strong><small>{phases.find((phase) => phase.status === app.status)?.label}</small></span>
                </Link>
              </li>
            ))}
          </ul>
        ) : (
          <div className="sp-bed-empty"><Plant status="undeployed" /><span>{isLoading ? "Looking at the soil…" : "The soil is ready."}</span></div>
        )}
        <div className="sp-bed-soil" aria-hidden="true" />
      </section>

      <ol className="sp-lifecycle" aria-label="Apps by lifecycle state">
        {phases.map((phase) => (
          <li className={`sp-phase sp-phase-${phase.status}${count(phase.status) ? " has-apps" : ""}`} key={phase.status} title={phase.hint}>
            <i aria-hidden="true" /><strong>{isLoading ? "–" : count(phase.status)}</strong><span>{phase.label}</span>
          </li>
        ))}
      </ol>

      {(isLoading || isError || sorted.length > 0) && <section className="sp-garden" aria-labelledby="sp-garden-title">
        <header className="sp-section-head">
          <h2 id="sp-garden-title">Your garden</h2>
          <span>Needs attention first</span>
        </header>

        {isLoading ? (
          <div className="sp-rows" aria-label="Loading applications">{[0, 1, 2].map((item) => <div className="sp-row sp-row-skeleton" key={item} />)}</div>
        ) : isError ? (
          <div className="sp-empty"><h3>Applications could not be loaded</h3><p>You can safely try again.</p><button className="sp-button sp-button-ghost" onClick={() => refetch()}>Try again</button></div>
        ) : (
          <ul className="sp-rows">{sorted.map((app) => <AppRow app={app} base={base} key={app.id} />)}</ul>
        )}
      </section>}
    </main>
  );
}

function AppRow({ app, base }: { app: Application; base: string }) {
  return (
    <li>
      <Link className={`sp-row sp-row-${app.status}`} href={`${base}/apps/${app.id}`}>
        <span className={`sp-monogram accent-${app.accent}`} aria-hidden="true">{app.name.slice(0, 2).toUpperCase()}</span>
        <span className="sp-row-main"><strong>{app.name}</strong><small className="sp-url">{app.url}</small></span>
        <StatusPill status={app.status} />
        <span className="sp-row-time">{app.updatedAt}</span>
        <ArrowRight size={15} aria-hidden="true" />
      </Link>
    </li>
  );
}
