"use client";

import { ArrowRight, Boxes, KeyRound, Users } from "lucide-react";
import Link from "next/link";
import { PageHeader } from "@/components/dashboard/page-header";
import { useApplications, useMe, useWorkspace, useWorkspaceMembers } from "@/lib/query/hooks";

const roleLabels = { owner: "Owner", editor: "Editor", viewer: "Viewer" } as const;

// What the workspace is and where to manage it. Renaming, leaving, and deleting a
// workspace need API support first, so this page only shows what Sprout can do today.
export function WorkspaceSettings({ workspaceSlug }: { workspaceSlug: string }) {
  const base = `/workspace/${workspaceSlug}`;
  const workspace = useWorkspace(workspaceSlug);
  const me = useMe();
  const members = useWorkspaceMembers(workspaceSlug);
  const apps = useApplications(workspaceSlug);
  const role = members.data?.find((member) => member.id === me.data?.id)?.role;
  const archived = apps.data?.filter((app) => app.status === "archived").length ?? 0;

  const links = [
    {
      href: `${base}/team`,
      icon: Users,
      title: "Team",
      detail: members.data ? `${members.data.length} ${members.data.length === 1 ? "person" : "people"} · invitations and roles` : "Members, invitations, and roles",
    },
    {
      href: `${base}/apps`,
      icon: Boxes,
      title: "Apps",
      detail: apps.data ? `${apps.data.length} ${apps.data.length === 1 ? "app" : "apps"}${archived ? ` · ${archived} archived` : ""}` : "Every app in this workspace",
    },
    {
      href: `${base}/tokens`,
      icon: KeyRound,
      title: "Access tokens",
      detail: "For the Sprout CLI, CI, and coding agents",
    },
  ];

  return (
    <main className="dashboard-page compact-page dash-stack">
      <PageHeader eyebrow="Settings" title="Workspace settings" description="What this workspace is, and where to manage the people and apps in it." />

      <section className="dash-panel-card">
        <div className="dash-panel-card-head"><div><h3>General</h3><p>How this workspace is identified.</p></div></div>
        <dl className="dash-definitions">
          <div><dt>Name</dt><dd>{workspace.data?.name ?? "…"}</dd></div>
          <div><dt>Address</dt><dd><code>/workspace/{workspaceSlug}</code></dd></div>
          <div><dt>Your role</dt><dd>{role ? <span className="dash-role">{roleLabels[role]}</span> : "…"}</dd></div>
        </dl>
        <p className="dash-panel-card-foot">Renaming, leaving, and deleting a workspace are not available yet.</p>
      </section>

      <ul className="dash-link-list">
        {links.map(({ href, icon: Icon, title, detail }, index) => (
          <li key={href} style={{ "--i": index } as React.CSSProperties}>
            <Link href={href}>
              <span className="dash-next-icon"><Icon size={15} aria-hidden="true" /></span>
              <span><strong>{title}</strong><small>{detail}</small></span>
              <ArrowRight size={14} aria-hidden="true" />
            </Link>
          </li>
        ))}
      </ul>
    </main>
  );
}
