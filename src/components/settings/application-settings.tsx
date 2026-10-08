"use client";

import { Archive, Pause, Play, Save, Server, TriangleAlert } from "lucide-react";
import { FormEvent } from "react";
import { useApplication, useUpdateApplication } from "@/lib/query/hooks";
import type { Application, ApplicationLifecycle } from "@/types/domain";

// Each lifecycle state offers only the transitions the API accepts.
const lifecycleActions: Record<ApplicationLifecycle, Array<{ to: ApplicationLifecycle; label: string; icon: typeof Play; effect: string }>> = {
  active: [
    { to: "paused", label: "Pause", icon: Pause, effect: "Stops new deployments. Access and configuration stay as they are." },
    { to: "archived", label: "Archive", icon: Archive, effect: "Retires the app from everyday use. You can restore it later." },
  ],
  paused: [
    { to: "active", label: "Resume", icon: Play, effect: "Allows deployments again." },
    { to: "archived", label: "Archive", icon: Archive, effect: "Retires the app from everyday use. You can restore it later." },
  ],
  archived: [
    { to: "active", label: "Restore", icon: Play, effect: "Brings the app back so it can be deployed again." },
  ],
};

const lifecycleLabels: Record<ApplicationLifecycle, string> = { active: "Active", paused: "Paused", archived: "Archived" };

export function ApplicationSettings({ workspaceSlug, appId }: { workspaceSlug: string; appId: string }) {
  const { data: application, isLoading, isError } = useApplication(workspaceSlug, appId);

  if (isLoading) return <div className="overview-loading"><span /><span /></div>;
  if (isError || !application) {
    return <section className="soft-empty"><TriangleAlert size={20} /><h2>Settings unavailable</h2><p>Sprout could not load this application with your current access.</p></section>;
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
    <section className="app-section settings-view">
      <header className="section-heading"><div><h2>Settings</h2><p>Name, description, and lifecycle for this application.</p></div></header>
      {update.isError && <p className="deploy-error" role="alert">{update.error.message}</p>}

      <form className="settings-panel" onSubmit={saveGeneral}>
        <div className="settings-panel-heading"><span className="panel-icon"><Server size={15} /></span><div><h3>General</h3><p>How this application appears to your team.</p></div></div>
        <div className="settings-fields">
          <label><span>Application name</span><input name="name" defaultValue={application.name} required maxLength={120} disabled={update.isPending} /></label>
          <label><span>Description</span><input name="description" defaultValue={application.description} maxLength={2000} disabled={update.isPending} /></label>
        </div>
        <div className="settings-actions">
          <span>URL name: {application.slug}</span>
          <button className="button button-primary" type="submit" disabled={update.isPending}><Save size={14} /> {update.isPending ? "Saving…" : "Save changes"}</button>
        </div>
      </form>

      <section className="settings-panel">
        <div className="settings-panel-heading"><span className="panel-icon"><Archive size={15} /></span><div><h3>Lifecycle</h3><p>Currently <strong>{lifecycleLabels[application.lifecycle]}</strong>.</p></div></div>
        <ul className="lifecycle-actions">
          {lifecycleActions[application.lifecycle].map(({ to, label, icon: Icon, effect }) => (
            <li key={to}>
              <div><strong>{label}</strong><small>{effect}</small></div>
              <button className="button button-secondary" type="button" disabled={update.isPending} onClick={() => update.mutate({ lifecycle: to })}>
                <Icon size={14} /> {label}
              </button>
            </li>
          ))}
        </ul>
      </section>
    </section>
  );
}
