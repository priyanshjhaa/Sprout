"use client";

import Link from "next/link";
import { useParams, usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { StatusPill } from "@/components/status-pill";
import { CopyAddress } from "@/components/copy-address";
import { Pipeline } from "@/components/pipeline";
import { Plant } from "@/components/plant";
import { useApplication } from "@/lib/query/hooks";
import { useLatestDeployment } from "@/lib/query/simulations";

const tabs = [
  { label: "Overview", path: "" },
  { label: "Deployments", path: "/deployments" },
  { label: "Access", path: "/access" },
  { label: "Settings", path: "/settings" },
];

export function ApplicationShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const params = useParams<{ workspaceSlug: string; appId: string }>();
  const workspaceSlug = params.workspaceSlug;
  const appId = params.appId;
  const base = `/workspace/${workspaceSlug}/apps/${appId}`;
  const { data: application, isLoading } = useApplication(workspaceSlug, appId);
  const latest = useLatestDeployment(workspaceSlug, appId);

  return (
    <main className="dashboard-page application-page sp-app dash-stack">
      <header className="dash-app-head" data-status={application?.status ?? "undeployed"}>
        <div className="dash-app-identity">
          <span className="dash-app-plant" aria-hidden="true">
            <Plant status={application?.status ?? "undeployed"} />
          </span>
          <div>
            <p className="dash-hero-eyebrow">App</p>
            <div className="dash-app-title">
              <h1>{isLoading ? "Loading application" : application?.name}</h1>
              {application && <StatusPill status={application.status} />}
            </div>
            {application && <CopyAddress url={application.url} live={application.status === "running"} />}
          </div>
        </div>
        <Pipeline stages={latest.job?.stages} simulated={latest.kind === "simulation"} />
      </header>

      <nav className="dash-tabs" aria-label="Application navigation">
        {tabs.map((tab) => {
          const href = `${base}${tab.path}`;
          const active = tab.path === "" ? pathname === base : pathname.startsWith(href);
          return (
            <Link aria-current={active ? "page" : undefined} href={href} key={tab.label}>
              {tab.label}
            </Link>
          );
        })}
      </nav>
      <div className="application-content">{children}</div>
    </main>
  );
}
