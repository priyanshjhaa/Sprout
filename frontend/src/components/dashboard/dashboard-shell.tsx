"use client";

import { SignOutButton } from "@clerk/nextjs";
import { ChevronLeft, Menu, Plus, X } from "lucide-react";
import Link from "next/link";
import { useParams, usePathname } from "next/navigation";
import { useState, type ReactNode } from "react";
import { WorkspaceSidebar } from "@/components/dashboard/workspace-sidebar";
import { useWorkspace } from "@/lib/query/hooks";

// The chip in the top bar names where you are, and links up a level when you are deeper.
function backChip(pathname: string, base: string): { label: string; href?: string } {
  const rest = pathname.slice(base.length).split("/").filter(Boolean);
  if (rest.length === 0) return { label: "Overview" };
  if (rest[0] === "apps" && rest.length > 1) return { label: "Apps", href: `${base}/apps` };
  const labels: Record<string, string> = {
    apps: "Apps",
    deploy: "Deploy",
    team: "Team",
    tokens: "Access tokens",
    settings: "Settings",
  };
  return { label: labels[rest[0]] ?? "Overview", href: base };
}

export function DashboardShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const params = useParams<{ workspaceSlug: string }>();
  const workspaceSlug = params.workspaceSlug ?? "acme";
  const base = `/workspace/${workspaceSlug}`;
  const { isError: workspaceUnavailable } = useWorkspace(workspaceSlug);
  // The drawer remembers which page it was opened on, so navigating closes it.
  const [drawerPath, setDrawerPath] = useState<string | null>(null);
  const drawerOpen = drawerPath === pathname;

  if (workspaceUnavailable) {
    return (
      <main className="auth-page">
        <section className="auth-card">
          <h1>Workspace unavailable</h1>
          <p>You may no longer have access to this workspace.</p>
          <Link className="button button-primary" href="/start">Open your workspace</Link>
          <SignOutButton redirectUrl="/">
            <button className="button button-secondary" type="button">Sign out</button>
          </SignOutButton>
        </section>
      </main>
    );
  }

  const chip = backChip(pathname, base);

  return (
    <div className="lp dash" data-drawer={drawerOpen || undefined}>
      <WorkspaceSidebar workspaceSlug={workspaceSlug} />
      <button
        type="button"
        className="dash-scrim"
        aria-label="Close navigation"
        tabIndex={drawerOpen ? 0 : -1}
        onClick={() => setDrawerPath(null)}
      />

      <div className="dash-panel">
        <header className="dash-topbar">
          {chip.href ? (
            <Link className="dash-chip" href={chip.href}>
              <ChevronLeft size={14} aria-hidden="true" /> {chip.label}
            </Link>
          ) : (
            <span className="dash-chip" aria-current="page">{chip.label}</span>
          )}
          <div className="dash-topbar-end">
            <Link className="lp-btn lp-btn--dark lp-btn--small" href={`${base}/deploy`}>
              <Plus size={15} aria-hidden="true" /> Deploy
            </Link>
            <button
              type="button"
              className="dash-icon-button dash-menu-button"
              aria-label={drawerOpen ? "Close navigation" : "Open navigation"}
              aria-expanded={drawerOpen}
              onClick={() => setDrawerPath(drawerOpen ? null : pathname)}
            >
              {drawerOpen ? <X size={17} /> : <Menu size={17} />}
            </button>
          </div>
        </header>
        <div className="sp-main">{children}</div>
      </div>
    </div>
  );
}
