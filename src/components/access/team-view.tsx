"use client";

import { Users } from "lucide-react";
import { useWorkspaceMembers } from "@/lib/query/hooks";

export function TeamView({ workspaceSlug }: { workspaceSlug: string }) {
  const members = useWorkspaceMembers(workspaceSlug);

  return <main className="dashboard-page compact-page">
    <header className="page-heading"><p className="eyebrow">Workspace</p><h1>Team</h1><p>The people who can build and use applications in this workspace.</p></header>
    <section className="members-panel">
      <header><span>Workspace members</span><small>{members.data?.length ?? 0} people</small></header>
      {members.isLoading && <p className="variable-loading" role="status">Loading team…</p>}
      {members.isError && <p className="variable-loading" role="alert">{members.error.message}</p>}
      {members.data?.map((member) => <div className="member-row" key={member.id}>
        <span className="member-avatar">{member.displayName.split(/\s+/).slice(0, 2).map((part) => part[0] ?? "").join("").toUpperCase()}</span>
        <div><strong>{member.displayName}</strong><small>{member.email}</small></div>
        <span className="member-inherited">{member.role[0].toUpperCase() + member.role.slice(1)}</span>
        <span aria-hidden="true" />
      </div>)}
    </section>
    <section className="soft-empty"><Users size={20} /><h2>Invitations are next</h2><p>For now, application access can be changed for people already in this workspace.</p></section>
  </main>;
}
