"use client";

import { SignOutButton, useClerk, useUser } from "@clerk/nextjs";
import { Boxes, ChevronDown, KeyRound, LayoutDashboard, LogOut, Rocket, Settings, UserCog, Users } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { CommandPalette } from "@/components/dashboard/command-palette";
import { useApplications, useWorkspace, useWorkspaces } from "@/lib/query/hooks";
import type { AppStatus } from "@/types/domain";

const statusLabel: Record<AppStatus, string> = {
  undeployed: "Not deployed",
  running: "Live",
  building: "Building",
  failed: "Needs attention",
  paused: "Paused",
  archived: "Archived",
};

export function WorkspaceSidebar({ workspaceSlug }: { workspaceSlug: string }) {
  const pathname = usePathname();
  const router = useRouter();
  const base = `/workspace/${workspaceSlug}`;
  const { data: workspace } = useWorkspace(workspaceSlug);
  const workspaces = useWorkspaces();
  const { data: apps } = useApplications(workspaceSlug);
  const { user } = useUser();
  const { openUserProfile } = useClerk();

  const main = [
    { label: "Overview", href: base, icon: LayoutDashboard, active: pathname === base },
    { label: "Deploy", href: `${base}/deploy`, icon: Rocket, active: pathname === `${base}/deploy` },
    { label: "Apps", href: `${base}/apps`, icon: Boxes, active: pathname.startsWith(`${base}/apps`), count: apps?.length },
    { label: "Team", href: `${base}/team`, icon: Users, active: pathname === `${base}/team` },
  ];
  const settings = [
    { label: "Workspace settings", href: `${base}/settings`, icon: Settings },
    { label: "Access tokens", href: `${base}/tokens`, icon: KeyRound },
  ];
  const recent = (apps ?? []).filter((app) => app.status !== "archived").slice(0, 6);
  const settingsOpen = settings.some((item) => pathname === item.href);
  const otherWorkspaces = workspaces.data?.filter((item) => item.slug !== workspaceSlug) ?? [];

  return (
    <aside className="dash-sidebar" aria-label="Workspace">
      <div className="dash-workspace">
        <span className="dash-logo" aria-hidden="true">
          <span className="brand-mark">
            <span />
            <span />
          </span>
        </span>
        <span className="dash-workspace-name">
          <b>{workspace?.name ?? "Your workspace"}</b>
          <small>Sprout workspace</small>
        </span>
      </div>

      {otherWorkspaces.length > 0 && (
        <label className="dash-switcher">
          <span className="sr-only">Switch workspace</span>
          <select
            value={workspaceSlug}
            onChange={(event) => router.push(`/workspace/${encodeURIComponent(event.target.value)}`)}
          >
            {!workspaces.data?.some((item) => item.slug === workspaceSlug) && (
              <option value={workspaceSlug}>{workspace?.name ?? "Your workspace"}</option>
            )}
            {workspaces.data?.map((item) => (
              <option key={item.id} value={item.slug}>
                {item.name}
              </option>
            ))}
          </select>
          <ChevronDown size={14} aria-hidden="true" />
        </label>
      )}

      <CommandPalette workspaceSlug={workspaceSlug} />

      <nav className="dash-nav" aria-label="Main">
        <p className="lp-mono">Main</p>
        {main.map(({ label, href, icon: Icon, active, count }, index) => (
          <Link
            key={href}
            href={href}
            className="dash-nav-link"
            aria-current={active ? "page" : undefined}
            style={{ "--i": index } as React.CSSProperties}
          >
            <Icon size={16} strokeWidth={1.8} aria-hidden="true" />
            <span>{label}</span>
            {count ? <em>{count}</em> : null}
          </Link>
        ))}
      </nav>

      <details className="dash-group" open>
        <summary>
          Recent apps <ChevronDown size={14} aria-hidden="true" />
        </summary>
        {recent.length ? (
          <ul className="dash-recent">
            {recent.map((app) => (
              <li key={app.id}>
                <Link
                  href={`${base}/apps/${app.id}`}
                  aria-current={pathname.startsWith(`${base}/apps/${app.id}`) ? "page" : undefined}
                >
                  <i data-status={app.status} aria-hidden="true" />
                  <span>{app.name}</span>
                  <span className="sr-only">, {statusLabel[app.status]}</span>
                </Link>
              </li>
            ))}
          </ul>
        ) : (
          <p className="dash-recent-empty">Apps you deploy show up here.</p>
        )}
      </details>

      <div className="dash-sidebar-foot">
        <details className="dash-group" open={settingsOpen || undefined}>
          <summary>
            <Settings size={15} aria-hidden="true" /> Settings <ChevronDown size={14} aria-hidden="true" />
          </summary>
          {settings.map(({ label, href, icon: Icon }) => (
            <Link key={href} href={href} className="dash-nav-link" aria-current={pathname === href ? "page" : undefined}>
              <Icon size={15} strokeWidth={1.8} aria-hidden="true" />
              <span>{label}</span>
            </Link>
          ))}
        </details>

        <details className="dash-user">
          <summary>
            {user?.imageUrl ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={user.imageUrl} alt="" width={32} height={32} />
            ) : (
              <span className="dash-user-fallback" aria-hidden="true" />
            )}
            <span className="dash-user-name">
              <b>{user?.fullName || user?.username || "Your account"}</b>
              <small>{user?.primaryEmailAddress?.emailAddress}</small>
            </span>
            <ChevronDown size={14} aria-hidden="true" />
          </summary>
          <div className="dash-user-menu">
            <button type="button" onClick={() => openUserProfile()}>
              <UserCog size={15} aria-hidden="true" /> Manage account
            </button>
            <SignOutButton redirectUrl="/">
              <button type="button">
                <LogOut size={15} aria-hidden="true" /> Sign out
              </button>
            </SignOutButton>
          </div>
        </details>
      </div>
    </aside>
  );
}
