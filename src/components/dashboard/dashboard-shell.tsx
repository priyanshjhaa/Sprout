"use client";

import {
  Activity,
  Bot,
  Boxes,
  ChevronDown,
  LogOut,
  Menu,
  Settings,
  Users,
  X,
} from "lucide-react";
import Link from "next/link";
import { SignOutButton, useUser, UserButton } from "@clerk/nextjs";
import { useParams, usePathname } from "next/navigation";
import { useState, type ReactNode } from "react";
import { Brand } from "@/components/brand";
import { useWorkspace } from "@/lib/query/hooks";

const primaryNavigation = [
  { label: "Agent", path: "agent", icon: Bot },
  { label: "Apps", path: "apps", icon: Boxes },
];

const secondaryNavigation = [
  { label: "Activity", path: "activity", icon: Activity },
  { label: "Team", path: "team", icon: Users },
  { label: "Settings", path: "settings", icon: Settings },
];

export function DashboardShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
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
        <div className="sidebar-brand"><Brand /></div>
        <button className="workspace-switcher" type="button">
          <span className="workspace-avatar">A</span>
          <span><strong>{workspace?.name ?? "Workspace"}</strong><small>Personal workspace</small></span>
          <ChevronDown size={15} />
        </button>

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

        <div className="sidebar-profile">
          <UserButton />
          <span><strong>{user?.fullName ?? user?.username ?? "Account"}</strong><small>{user?.primaryEmailAddress?.emailAddress ?? "Signed in"}</small></span>
        </div>
        <SignOutButton redirectUrl="/">
          <button className="sidebar-link sidebar-sign-out" type="button"><LogOut size={17} strokeWidth={1.8} /><span>Sign out</span></button>
        </SignOutButton>
      </aside>

      {mobileOpen && <button className="sidebar-backdrop" aria-label="Close navigation" onClick={() => setMobileOpen(false)} />}
      <div className="dashboard-content">{children}</div>
    </div>
  );
}
