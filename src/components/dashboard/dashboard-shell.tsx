"use client";

import { Boxes, LayoutDashboard, Rocket, Sprout, Users } from "lucide-react";
import Link from "next/link";
import { SignOutButton, UserButton } from "@clerk/nextjs";
import { useParams, usePathname, useRouter } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";
import { CommandPalette } from "@/components/dashboard/command-palette";
import { Brand } from "@/components/brand";
import { useWorkspace, useWorkspaces } from "@/lib/query/hooks";

const navigationItems = [
  { label: "Overview", path: "", icon: LayoutDashboard },
  { label: "Apps", path: "apps", icon: Boxes },
  { label: "Team", path: "team", icon: Users },
];

type Daypart = "dawn" | "day" | "dusk" | "night";

function daypartAt(date: Date): Daypart {
  const hour = date.getHours();
  return hour >= 5 && hour < 9 ? "dawn" : hour >= 9 && hour < 17 ? "day" : hour >= 17 && hour < 20 ? "dusk" : "night";
}

// The sky follows the visitor's clock. It starts as "day" so server and client
// markup match, then settles on the real hour after mounting.
function useDaypart(): Daypart {
  const [daypart, setDaypart] = useState<Daypart>("day");
  useEffect(() => {
    const update = () => setDaypart(daypartAt(new Date()));
    update();
    const timer = window.setInterval(update, 10 * 60 * 1000);
    return () => window.clearInterval(timer);
  }, []);
  return daypart;
}

export function DashboardShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const workspaces = useWorkspaces();
  const params = useParams<{ workspaceSlug: string }>();
  const workspaceSlug = params.workspaceSlug ?? "acme";
  const daypart = useDaypart();
  const { data: workspace, isError: workspaceUnavailable } = useWorkspace(workspaceSlug);

  const navigation = (items: typeof navigationItems) =>
    items.map(({ label, path, icon: Icon }) => {
      const href = path ? `/workspace/${workspaceSlug}/${path}` : `/workspace/${workspaceSlug}`;
      const active = pathname === href || (path === "apps" && pathname.startsWith(`${href}/`));
      return (
        <Link
          key={path}
          className={`sp-bar-link${active ? " active" : ""}`}
          href={href}
        >
          <Icon size={15} strokeWidth={1.9} />
          <span>{label}</span>
        </Link>
      );
    });

  if (workspaceUnavailable) {
    return <main className="auth-page"><section className="auth-card"><h1>Workspace unavailable</h1><p>You may no longer have access to this workspace.</p><Link className="button button-primary" href="/start">Open your workspace</Link><SignOutButton redirectUrl="/"><button className="button button-secondary" type="button">Sign out</button></SignOutButton></section></main>;
  }

  return (
    <div className="sp-frame" data-daypart={daypart}>
      <div className="sp-sky" aria-hidden="true" />
      <div className="sp-grain" aria-hidden="true" />
      <header className="sp-bar">
        <Brand />
        <nav className="sp-bar-nav" aria-label="Workspace navigation">
          {navigation(navigationItems)}
        </nav>
        <div className="sp-bar-end">
          <CommandPalette workspaceSlug={workspaceSlug} />
          <div className="sp-workspace">
            <span className="sp-workspace-mark"><Sprout size={15} strokeWidth={2} /></span>
            <label><span className="sr-only">Switch workspace</span>
              <select value={workspaceSlug} aria-label="Switch workspace" disabled={workspaces.isLoading || workspaces.isError} onChange={(event) => router.push(`/workspace/${encodeURIComponent(event.target.value)}`)}>
                {!workspaces.data?.some((item) => item.slug === workspaceSlug) && <option value={workspaceSlug}>{workspace?.name ?? "Your workspace"}</option>}
                {workspaces.data?.map((item) => <option value={item.slug} key={item.id}>{item.name}</option>)}
              </select>
              <small>{workspaces.isError ? "Could not load workspaces" : "Workspace"}</small>
            </label>
          </div>
          <Link className="sp-button sp-button-solid" href={`/workspace/${workspaceSlug}/deploy`}>
            <Rocket size={15} strokeWidth={2.2} /> Deploy
          </Link>
          <UserButton />
        </div>
      </header>

      <div className="sp-main">
        {children}
      </div>
    </div>
  );
}
