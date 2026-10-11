"use client";

import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { useParams, usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { StatusPill } from "@/components/status-pill";
import { Pipeline } from "@/components/pipeline";
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
    <main className="dashboard-page application-page sp-app">
      <Link className="app-back-link" href={`/workspace/${workspaceSlug}/apps`}><ArrowLeft size={14} /> All apps</Link>
      <header className={`sp-app-hero sp-app-hero-${application?.status ?? "undeployed"}`}>
        <div className="sp-app-identity">
          <span className={`sp-monogram sp-monogram-lg accent-${application?.accent ?? "amber"}`}>{application?.name.slice(0, 2).toUpperCase() ?? "··"}</span>
          <div>
            <div className="sp-app-title"><h1>{isLoading ? "Loading application" : application?.name}</h1>{application && <StatusPill status={application.status} />}</div>
            {application && <span className="sp-app-url">{application.status === "running" ? "Live at" : "Reserved address"} · {application.url}</span>}
          </div>
        </div>
        <Pipeline stages={latest.job?.stages} />
      </header>

      <nav className="sp-tabs" aria-label="Application navigation">
        {tabs.map((tab) => {
          const href = `${base}${tab.path}`;
          const active = tab.path === "" ? pathname === base : pathname.startsWith(href);
          return <Link className={active ? "active" : ""} href={href} key={tab.label}>{tab.label}</Link>;
        })}
      </nav>
      <div className="application-content">{children}</div>
    </main>
  );
}
