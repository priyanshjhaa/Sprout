"use client";

import { useAuth } from "@clerk/nextjs";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link2, ShieldCheck } from "lucide-react";
import { api } from "@/lib/api/api";
import { useAccess, useWorkspaceMembers } from "@/lib/query/hooks";
import { queryKeys } from "@/lib/query/keys";

type AccessChange =
  | { kind: "mode"; mode: "workspace" | "restricted" }
  | { kind: "grant"; memberID: string; role: "editor" | "viewer" }
  | { kind: "revoke"; memberID: string };

function initials(name: string): string {
  return name.split(/\s+/).slice(0, 2).map((part) => part[0] ?? "").join("").toUpperCase();
}

export function AccessView({ workspaceSlug, appId }: { workspaceSlug: string; appId: string }) {
  const { getToken, userId } = useAuth();
  const queryClient = useQueryClient();
  const access = useAccess(workspaceSlug, appId);
  const members = useWorkspaceMembers(workspaceSlug);
  const change = useMutation({
    mutationFn: async (action: AccessChange) => {
      const token = await getToken();
      if (action.kind === "mode") return api.setApplicationAccessMode(workspaceSlug, appId, action.mode, token);
      if (action.kind === "grant") return api.grantApplicationAccess(workspaceSlug, appId, action.memberID, action.role, token);
      return api.revokeApplicationAccess(workspaceSlug, appId, action.memberID, token);
    },
    onSuccess: async (updated) => {
      queryClient.setQueryData(queryKeys.access(userId, workspaceSlug, appId), updated);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.app(userId, workspaceSlug, appId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.apps(userId, workspaceSlug) }),
      ]);
    },
  });
  const mode = access.data?.accessMode;

  return (
    <section className="app-section access-view">
      <header className="section-heading">
        <div><h2>Access</h2><p>Choose who can see and manage this application in Sprout.</p></div>
      </header>

      {access.isLoading && <p className="variable-loading" role="status">Loading access…</p>}
      {access.isError && <p className="variable-loading" role="alert">{access.error.message}</p>}
      {change.isError && <p className="variable-loading" role="alert">{change.error.message}</p>}

      {access.data && <>
        <section className="workspace-access-card">
          <span className="panel-icon"><ShieldCheck size={16} /></span>
          <div>
            <h3>Workspace access</h3>
            <p>{mode === "workspace" ? "Every member can view its Sprout page. Workspace editors can change it." : "Only the creator, workspace owners, and people granted access can view its Sprout page."}</p>
          </div>
          <button
            className={`toggle${mode === "workspace" ? " on" : ""}`}
            type="button"
            role="switch"
            aria-label="Allow all workspace members to access this application"
            aria-checked={mode === "workspace"}
            disabled={change.isPending}
            onClick={() => change.mutate({ kind: "mode", mode: mode === "workspace" ? "restricted" : "workspace" })}
          ><span /></button>
        </section>

        <section className="members-panel">
          <header><span>Workspace members</span><small>{members.data?.length ?? 0} people</small></header>
          <div className="member-list">
            {members.isLoading && <p className="variable-loading" role="status">Loading members…</p>}
            {members.isError && <p className="variable-loading" role="alert">{members.error.message}</p>}
            {members.data?.map((member) => {
              const grant = access.data.grants.find((item) => item.id === member.id);
              const inherited = member.role === "owner" || member.id === access.data.createdBy;
              return <div className="member-row" key={member.id}>
                <span className="member-avatar">{initials(member.displayName)}</span>
                <div><strong>{member.displayName}</strong><small>{member.email}</small></div>
                {inherited ? <span className="member-inherited">{member.role === "owner" ? "Owner" : "Creator"}</span> :
                  <select
                    aria-label={`Application access for ${member.displayName}`}
                    value={grant?.role ?? ""}
                    disabled={change.isPending}
                    onChange={(event) => {
                      const role = event.target.value;
                      change.mutate(role === "" ? { kind: "revoke", memberID: member.id } :
                        { kind: "grant", memberID: member.id, role: role as "editor" | "viewer" });
                    }}
                  >
                    <option value="">{mode === "workspace" ? "Workspace role" : "No access"}</option>
                    <option value="viewer">Viewer</option>
                    <option value="editor">Editor</option>
                  </select>}
                <span aria-hidden="true" />
              </div>;
            })}
          </div>
          <footer><Link2 size={12} /> These controls do not secure a deployed app&apos;s own URL. Invite new members from the Team page.</footer>
        </section>
      </>}
    </section>
  );
}
