"use client";

import { useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api/api";
import { useInvitations, useMe, useWorkspaceMembers } from "@/lib/query/hooks";
import { queryKeys } from "@/lib/query/keys";

type TeamAction = { kind: "invite"; email: string; role: "editor" | "viewer" }
  | { kind: "role"; id: string; role: "editor" | "viewer" }
  | { kind: "remove" | "revoke"; id: string };

export function TeamView({ workspaceSlug }: { workspaceSlug: string }) {
  const members = useWorkspaceMembers(workspaceSlug);
  const me = useMe();
  const isOwner = members.data?.find((member) => member.id === me.data?.id)?.role === "owner";
  const invitations = useInvitations(workspaceSlug, isOwner);
  const { getToken, userId } = useAuth();
  const client = useQueryClient();
  const [link, setLink] = useState("");
  const [notice, setNotice] = useState("");
  const [removing, setRemoving] = useState<string | null>(null);
  const change = useMutation({
    gcTime: 0,
    mutationFn: async (action: TeamAction) => {
      const token = await getToken();
      if (action.kind === "invite") {
        const created = await api.createInvitation(workspaceSlug, action.email, action.role, token);
        // Keep the raw link in component memory, never in the query cache.
        setLink(`${window.location.origin}/invitations/accept#${created.token}`);
        return;
      }
      if (action.kind === "role") await api.changeMemberRole(workspaceSlug, action.id, action.role, token);
      else if (action.kind === "remove") await api.removeMember(workspaceSlug, action.id, token);
      else await api.revokeInvitation(workspaceSlug, action.id, token);
    },
    onMutate: async () => {
      setNotice(""); setLink("");
      await client.cancelQueries({ queryKey: queryKeys.workspace(userId, workspaceSlug) });
    },
    onSuccess: async (_, action) => {
      setRemoving(null);
      setNotice(action.kind === "invite" ? "Invitation created. Share the link privately; it expires in 7 days." : "Team updated.");
      await client.invalidateQueries({ queryKey: queryKeys.workspace(userId, workspaceSlug) });
    },
  });

  return <main className="dashboard-page compact-page">
    <header className="page-heading"><h1>Team</h1><p>People in this workspace and what they can do.</p></header>
    {me.isError && <p role="alert">{me.error.message}</p>}
    {change.isError && <p role="alert">{change.error.message}</p>}
    {notice && <p role="status">{notice}</p>}
    {isOwner && <section className="team-invite-panel">
      <h2>Invite a teammate</h2><p>Editors can create, deploy, and pause apps. Viewers can see them in Sprout. Only owners manage the team.</p>
      <form className="team-invite-form" onSubmit={(event) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        change.mutate({ kind: "invite", email: String(data.get("email")), role: data.get("role") as "editor" | "viewer" });
      }}>
        <label>Email address<input name="email" type="email" required maxLength={320} placeholder="teammate@example.com" disabled={change.isPending} /></label>
        <label>Workspace role<select name="role" defaultValue="viewer" disabled={change.isPending}><option value="viewer">Viewer</option><option value="editor">Editor</option></select></label>
        <button className="button button-primary" disabled={change.isPending}>Create invitation</button>
      </form>
      <small>No email is sent. The recipient must sign in with this verified primary email.</small>
      {link && <div className="team-share-link"><label>Copy this link now — it is shown only once<input readOnly value={link} onFocus={(event) => event.target.select()} /></label><button className="button button-secondary" onClick={async () => {
        try { await navigator.clipboard.writeText(link); setNotice("Invitation link copied."); }
        catch { setNotice("Select the link above and copy it manually."); }
      }}>Copy link</button></div>}
    </section>}
    <section className="members-panel">
      <header><span>Workspace members</span><small>{members.data?.length === 1 ? "1 person" : `${members.data?.length ?? 0} people`}</small></header>
      {members.isLoading && <p className="team-panel-note" role="status">Loading team…</p>}
      {members.isError && <p className="team-panel-note" role="alert">{members.error.message}</p>}
      {members.data?.map((member) => <div className="member-row team-member-row" key={member.id}>
        <span className="member-avatar">{member.displayName.split(/\s+/).slice(0, 2).map((part) => part[0] ?? "").join("").toUpperCase()}</span>
        <div><strong>{member.displayName}</strong><small>{member.email}</small></div>
        {isOwner && member.role !== "owner" ? <>
          <select aria-label={`Role for ${member.displayName}`} value={member.role} disabled={change.isPending} onChange={(event) => change.mutate({ kind: "role", id: member.id, role: event.target.value as "editor" | "viewer" })}><option value="editor">Editor</option><option value="viewer">Viewer</option></select>
          <button className="button button-secondary" disabled={change.isPending} onClick={() => setRemoving(member.id)}>Remove<span className="sr-only"> {member.displayName}</span></button>
          {removing === member.id && <div className="team-remove-confirm" role="group" aria-label={`Confirm removal of ${member.displayName}`}><p>Remove {member.displayName}? They will lose workspace access and their application grants.</p><button className="button button-secondary" disabled={change.isPending} onClick={() => setRemoving(null)}>Cancel</button><button className="button button-primary" disabled={change.isPending} onClick={() => change.mutate({ kind: "remove", id: member.id })}>Confirm removal</button></div>}
        </> : <span className="member-inherited">{member.role.charAt(0).toUpperCase() + member.role.slice(1)}</span>}
      </div>)}
    </section>
    {/* Pending invitations only take space when there is something to act on. */}
    {isOwner && (invitations.isError || (invitations.data?.length ?? 0) > 0) && <section className="members-panel">
      <header><span>Pending invitations</span><small>Links expire after 7 days</small></header>
      {invitations.isError && <p className="team-panel-note" role="alert">{invitations.error.message}</p>}
      {invitations.data?.map((invite) => <div className="team-invitation-row" key={invite.id}><div><strong>{invite.email}</strong><small>{invite.role} · Expires {new Date(invite.expiresAt).toLocaleDateString()}</small></div><button className="button button-secondary" disabled={change.isPending} onClick={() => change.mutate({ kind: "revoke", id: invite.id })}>Revoke<span className="sr-only"> invitation for {invite.email}</span></button></div>)}
    </section>}
    {!isOwner && members.data && <p className="muted">Your workspace owner can invite people and manage membership.</p>}
  </main>;
}
