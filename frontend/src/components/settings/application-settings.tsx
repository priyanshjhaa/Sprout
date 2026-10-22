"use client";

import { Archive, Pause, Play, Save, Server, TriangleAlert } from "lucide-react";
import { FormEvent } from "react";
import { useApplication, useUpdateApplication } from "@/lib/query/hooks";
import type { Application, ApplicationLifecycle } from "@/types/domain";

// Each lifecycle state offers only the transitions the API accepts.
const lifecycleActions: Record<ApplicationLifecycle, Array<{ to: ApplicationLifecycle; label: string; icon: typeof Play; effect: string }>> = {
  active: [
    { to: "paused", label: "Pause", icon: Pause, effect: "Takes the live URL offline and stops new deployments. Resume brings it back." },
    { to: "archived", label: "Archive", icon: Archive, effect: "Takes the live URL offline and removes the running copy. You can restore it, then deploy again." },
  ],
  paused: [
    { to: "active", label: "Resume", icon: Play, effect: "Puts the live URL back; the app starts on its next visit." },
    { to: "archived", label: "Archive", icon: Archive, effect: "Takes the live URL offline and removes the running copy. You can restore it, then deploy again." },
  ],
  archived: [
    { to: "active", label: "Restore", icon: Play, effect: "Brings the app back. Its URL answers again after you deploy." },
  ],
};

const lifecycleLabels: Record<ApplicationLifecycle, string> = { active: "Active", paused: "Paused", archived: "Archived" };

export function ApplicationSettings({ workspaceSlug, appId }: { workspaceSlug: string; appId: string }) {
  const { data: application, isLoading, isError } = useApplication(workspaceSlug, appId);

  if (isLoading) return <div className="dash-loading" aria-label="Loading settings"><span /><span /></div>;
  if (isError || !application) {
    return <section className="dash-empty"><TriangleAlert size={20} aria-hidden="true" /><h3>Settings unavailable</h3><p>Sprout could not load this application with your current access.</p></section>;
  }
  // Re-mount the form when the saved values change so inputs show the latest data.
  return <SettingsForm key={application.updatedAt + application.name} workspaceSlug={workspaceSlug} application={application} />;
}

function SettingsForm({ workspaceSlug, application }: { workspaceSlug: string; application: Application }) {
  const update = useUpdateApplication(workspaceSlug, application.id);

  function saveGeneral(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const name = String(data.get("name")).trim();
    const description = String(data.get("description")).trim();
    const changes = {
      ...(name !== application.name ? { name } : {}),
      ...(description !== application.description ? { description } : {}),
    };
    if (Object.keys(changes).length > 0) update.mutate(changes);
  }

  return (
    <section className="dash-stack">
      <header className="dash-subhead"><div><h2>Settings</h2><p>Name, description, and lifecycle for this application.</p></div></header>
      {update.isError && <p className="dash-callout dash-callout--error" role="alert">{update.error.message}</p>}

      <form className="dash-panel-card" onSubmit={saveGeneral}>
        <div className="dash-panel-card-head"><span className="dash-next-icon"><Server size={15} aria-hidden="true" /></span><div><h3>General</h3><p>How this application appears to your team.</p></div></div>
        <div className="dash-fields">
          <label><span>Application name</span><input name="name" defaultValue={application.name} required maxLength={120} disabled={update.isPending} /></label>
          <label><span>Description</span><input name="description" defaultValue={application.description} maxLength={2000} disabled={update.isPending} /></label>
        </div>
        <div className="dash-panel-card-foot dash-form-actions">
          <span>URL name: <code>{application.slug}</code></span>
          <button className="lp-btn lp-btn--dark lp-btn--small" type="submit" disabled={update.isPending}><Save size={14} aria-hidden="true" /> {update.isPending ? "Saving…" : "Save changes"}</button>
        </div>
      </form>

      <section className="dash-panel-card">
        <div className="dash-panel-card-head"><span className="dash-next-icon"><Archive size={15} aria-hidden="true" /></span><div><h3>Lifecycle</h3><p>Currently <strong>{lifecycleLabels[application.lifecycle]}</strong>.</p></div></div>
        <ul className="dash-actions-list">
          {lifecycleActions[application.lifecycle].map(({ to, label, icon: Icon, effect }) => (
            <li key={to} data-to={to}>
              <div><strong>{label}</strong><small>{effect}</small></div>
              <button className="lp-btn lp-btn--light lp-btn--small" type="button" disabled={update.isPending} onClick={() => update.mutate({ lifecycle: to })}>
                <Icon size={14} aria-hidden="true" /> {label}
              </button>
            </li>
          ))}
        </ul>
      </section>
    </section>
  );
}
