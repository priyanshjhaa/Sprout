"use client";

import { Boxes, LayoutDashboard, LogOut, Menu, Rocket, Sprout, Users, X } from "lucide-react";
import Link from "next/link";
import { SignOutButton, useUser, UserButton } from "@clerk/nextjs";
import { useParams, usePathname, useRouter } from "next/navigation";
import { useState, type ReactNode } from "react";
import { Brand } from "@/components/brand";
import { useWorkspace, useWorkspaces } from "@/lib/query/hooks";

const navigationItems = [
  { label: "Overview", path: "", icon: LayoutDashboard },
  { label: "Apps", path: "apps", icon: Boxes },
  { label: "Team", path: "team", icon: Users },
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

  const navigation = (items: typeof navigationItems) =>
    items.map(({ label, path, icon: Icon }) => {
      const href = path ? `/workspace/${workspaceSlug}/${path}` : `/workspace/${workspaceSlug}`;
      const active = pathname === href || (path === "apps" && pathname.startsWith(`${href}/`));
      return (
        <Link
          key={path}
          className={`sp-nav-link${active ? " active" : ""}`}
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
    <div className="sp-frame">
      <header className="sp-mobile-bar">
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

      <aside className={`sp-side${mobileOpen ? " open" : ""}`}>
        <Brand />
        <div className="sp-workspace">
          <span className="sp-workspace-mark"><Sprout size={16} strokeWidth={2} /></span>
          <label><span className="sr-only">Switch workspace</span>
            <select value={workspaceSlug} aria-label="Switch workspace" disabled={workspaces.isLoading || workspaces.isError} onChange={(event) => { setMobileOpen(false); router.push(`/workspace/${encodeURIComponent(event.target.value)}`); }}>
              {!workspaces.data?.some((item) => item.slug === workspaceSlug) && <option value={workspaceSlug}>{workspace?.name ?? "Your workspace"}</option>}
              {workspaces.data?.map((item) => <option value={item.slug} key={item.id}>{item.name}</option>)}
            </select>
            <small>{workspaces.isError ? "Could not load workspaces" : "Workspace"}</small>
          </label>
        </div>

        <Link className="sp-deploy-cta" href={`/workspace/${workspaceSlug}/deploy`} onClick={() => setMobileOpen(false)}>
          <Rocket size={16} strokeWidth={2.2} /> Deploy an app
        </Link>

        <nav className="sp-nav" aria-label="Workspace navigation">
          {navigation(navigationItems)}
        </nav>

        <div className="sp-account">
          <div className="sp-profile">
            <UserButton />
            <span><strong>{user?.fullName ?? user?.username ?? "Account"}</strong><small>{user?.primaryEmailAddress?.emailAddress ?? "Signed in"}</small></span>
          </div>
          <SignOutButton redirectUrl="/">
            <button className="sp-nav-link" type="button"><LogOut size={17} strokeWidth={1.8} /><span>Sign out</span></button>
          </SignOutButton>
        </div>
      </aside>

      {mobileOpen && <button className="sp-backdrop" aria-label="Close navigation" onClick={() => setMobileOpen(false)} />}
      <div className="sp-main">
        {children}
      </div>
    </div>
  );
}
