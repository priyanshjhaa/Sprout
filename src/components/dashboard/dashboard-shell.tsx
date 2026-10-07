"use client";

import {
  Activity,
  Rocket,
  Boxes,
  LogOut,
  Menu,
  Settings,
  Sprout,
  Users,
  X,
} from "lucide-react";
import Link from "next/link";
import { SignOutButton, useUser, UserButton } from "@clerk/nextjs";
import { useParams, usePathname, useRouter } from "next/navigation";
import { useState, type ReactNode } from "react";
import { Brand } from "@/components/brand";
import { useWorkspace, useWorkspaces } from "@/lib/query/hooks";

const primaryNavigation = [
  { label: "Deploy", path: "deploy", icon: Rocket },
  { label: "Apps", path: "apps", icon: Boxes },
];

const secondaryNavigation = [
  { label: "Activity", path: "activity", icon: Activity },
  { label: "Team", path: "team", icon: Users },
  { label: "Settings", path: "settings", icon: Settings },
];

export function DashboardShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const workspaces = useWorkspaces();
  const params = useParams<{ workspaceSlug: string }>();
  const workspaceSlug = params.workspaceSlug ?? "acme";
  const { data: workspace, isError: workspaceUnavailable } = useWorkspace(workspaceSlug);
  const { user } = useUser();
  const [mobileOpen, setMobileOpen] = useState(false);

  const navigation = (items: typeof primaryNavigation) =>
    items.map(({ label, path, icon: Icon }) => {
      const href = `/workspace/${workspaceSlug}/${path}`;
      const active = pathname === href || (path === "apps" && pathname.startsWith(`${href}/`));
      return (
        <Link
          key={path}
          className={`sidebar-link${active ? " active" : ""}`}
          href={href}
          onClick={() => setMobileOpen(false)}
        >
          <Icon size={17} strokeWidth={1.8} />
          <span>{label}</span>
        </Link>
      );
    });

  if (workspaceUnavailable) {
    return <main className="auth-page"><section className="auth-card"><h1>Workspace unavailable</h1><p>You may no longer have access to this workspace.</p><Link className="button button-primary" href="/start">Open your workspace</Link><SignOutButton redirectUrl="/"><button className="button button-secondary" type="button">Sign out</button></SignOutButton></section></main>;
  }

  return (
    <div className="dashboard-frame">
      <header className="mobile-dashboard-header">
        <Brand />
        <button
          className="icon-button"
          type="button"
          aria-label={mobileOpen ? "Close navigation" : "Open navigation"}
          aria-expanded={mobileOpen}
          onClick={() => setMobileOpen((open) => !open)}
        >
          {mobileOpen ? <X size={19} /> : <Menu size={19} />}
        </button>
      </header>

      <aside className={`dashboard-sidebar${mobileOpen ? " mobile-open" : ""}`}>
        <div className="sidebar-brand"><Brand /><span>SMALL SOFTWARE, AT HOME</span></div>
        <div className="workspace-switcher">
          <span className="workspace-avatar"><Sprout size={17} strokeWidth={1.7} /></span>
          <label className="workspace-picker"><span className="sr-only">Switch workspace</span>
            <select value={workspaceSlug} aria-label="Switch workspace" disabled={workspaces.isLoading || workspaces.isError} onChange={(event) => { setMobileOpen(false); router.push(`/workspace/${encodeURIComponent(event.target.value)}/apps`); }}>
              {!workspaces.data?.some((item) => item.slug === workspaceSlug) && <option value={workspaceSlug}>{workspace?.name ?? "Your workspace"}</option>}
              {workspaces.data?.map((item) => <option value={item.slug} key={item.id}>{item.name}</option>)}
            </select><small>{workspaces.isError ? "Could not load workspaces" : "Your workspaces"}</small>
          </label>
        </div>

        <nav className="sidebar-navigation" aria-label="Workspace navigation">
          <div className="sidebar-group">
            <p>Workspace</p>
            {navigation(primaryNavigation)}
          </div>
          <div className="sidebar-group sidebar-group-secondary">
            <p>Manage</p>
            {navigation(secondaryNavigation)}
          </div>
        </nav>

        <div className="sidebar-account">
          <div className="sidebar-profile">
            <UserButton />
            <span><strong>{user?.fullName ?? user?.username ?? "Account"}</strong><small>{user?.primaryEmailAddress?.emailAddress ?? "Signed in"}</small></span>
          </div>
          <SignOutButton redirectUrl="/">
            <button className="sidebar-link sidebar-sign-out" type="button"><LogOut size={17} strokeWidth={1.8} /><span>Sign out</span></button>
          </SignOutButton>
        </div>
      </aside>

      {mobileOpen && <button className="sidebar-backdrop" aria-label="Close navigation" onClick={() => setMobileOpen(false)} />}
      <div className="dashboard-content">
        <header className="dashboard-topbar">
          <div className="dashboard-topbar-path"><i aria-hidden="true" /><span>Workspace</span><span aria-hidden="true">/</span><strong>{workspace?.name ?? "Your workspace"}</strong></div>
          <span className="dashboard-topbar-state"><i aria-hidden="true" /> A home for small software</span>
        </header>
        {children}
      </div>
    </div>
  );
}
