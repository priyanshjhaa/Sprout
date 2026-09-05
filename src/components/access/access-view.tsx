"use client";

import { Link2, Mail, Plus, ShieldCheck, UserMinus, X } from "lucide-react";
import { FormEvent, useState } from "react";
import { useAccess } from "@/lib/query/hooks";
import type { Member } from "@/types/domain";

export function AccessView({ workspaceSlug, appId }: { workspaceSlug: string; appId: string }) {
  const { data: initialMembers, isLoading } = useAccess(workspaceSlug, appId);
  const [invited, setInvited] = useState<Member[]>([]);
  const [removed, setRemoved] = useState<string[]>([]);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [workspaceAccess, setWorkspaceAccess] = useState(true);
  const members = [...(initialMembers ?? []), ...invited].filter((member) => !removed.includes(member.id));

  function invite(event: FormEvent) {
    event.preventDefault();
    const address = email.trim();
    if (!address || !address.includes("@")) return;
    setInvited((items) => [...items, { id: `invited-${address}`, name: address.split("@")[0], email: address, role: "Viewer", initials: address.slice(0, 2).toUpperCase() }]);
    setEmail("");
    setInviteOpen(false);
  }

  return (
    <section className="app-section access-view">
      <header className="section-heading"><div><h2>Access</h2><p>Share this application without rebuilding identity inside it.</p></div><button className="button button-primary" type="button" onClick={() => setInviteOpen(true)}><Plus size={14} /> Invite people</button></header>

      {inviteOpen && <form className="invite-form" onSubmit={invite}>
        <Mail size={15} />
        <label><span className="sr-only">Email address</span><input autoFocus type="email" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="teammate@company.com" /></label>
        <select aria-label="Initial role" defaultValue="Viewer"><option>Viewer</option><option>Editor</option></select>
        <button className="button button-primary" type="submit">Send invite</button>
        <button className="icon-button" type="button" onClick={() => setInviteOpen(false)} aria-label="Close invitation form"><X size={15} /></button>
      </form>}

      <section className="workspace-access-card">
        <span className="panel-icon"><ShieldCheck size={16} /></span>
        <div><h3>Workspace access</h3><p>Anyone in Acme studio can open this application. Individual roles below still control who can change it.</p></div>
        <button className={`toggle${workspaceAccess ? " on" : ""}`} type="button" role="switch" aria-checked={workspaceAccess} onClick={() => setWorkspaceAccess((value) => !value)}><span /></button>
      </section>

      <section className="members-panel">
        <header><span>People with access</span><small>{members.length} members</small></header>
        <div className="member-list">
          {isLoading ? <div className="variable-loading">Loading access…</div> : members.map((member) => (
            <div className="member-row" key={member.id}>
              <span className="member-avatar">{member.initials}</span>
              <div><strong>{member.name}</strong><small>{member.email}</small></div>
              <select aria-label={`Role for ${member.name}`} defaultValue={member.role} disabled={member.role === "Owner"}><option>Owner</option><option>Editor</option><option>Viewer</option></select>
              {member.role !== "Owner" && <button type="button" aria-label={`Remove ${member.name}`} onClick={() => setRemoved((items) => [...items, member.id])}><UserMinus size={14} /></button>}
            </div>
          ))}
        </div>
        <footer><Link2 size={12} /> Identity is inherited from the workspace and applied before the app loads.</footer>
      </section>
    </section>
  );
}

