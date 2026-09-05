"use client";

import { ArrowLeft, ExternalLink, MoreHorizontal } from "lucide-react";
import Link from "next/link";
import { useParams, usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { StatusPill } from "@/components/status-pill";
import { useApplication } from "@/lib/query/hooks";

const tabs = [
  { label: "Overview", path: "" },
  { label: "Deployments", path: "/deployments" },
  { label: "Logs", path: "/logs" },
  { label: "Environment", path: "/environment" },
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

  return (
    <main className="dashboard-page application-page">
      <Link className="app-back-link" href={`/workspace/${workspaceSlug}/apps`}><ArrowLeft size={14} /> All apps</Link>
      <header className="application-header">
        <div className="application-identity">
          <span className="application-icon">{application?.name.slice(0, 2).toUpperCase() ?? "··"}</span>
          <div>
            <div className="application-title-row">
              <h1>{isLoading ? "Loading application" : application?.name}</h1>
              {application && <StatusPill status={application.status} />}
            </div>
            <span>{application?.url ?? "Fetching address…"}</span>
          </div>
        </div>
        <div className="application-actions">
          {application && <a className="button button-secondary" href={`https://${application.url}`} target="_blank" rel="noreferrer">Open app <ExternalLink size={14} /></a>}
          <button className="icon-button" type="button" aria-label="More application actions"><MoreHorizontal size={17} /></button>
        </div>
      </header>

      <nav className="application-tabs" aria-label="Application navigation">
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

