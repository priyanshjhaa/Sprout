"use client";

import { ArrowRight, Boxes, Plus, Search } from "lucide-react";
import Link from "next/link";
import { useMemo, useState } from "react";
import type { AppStatus } from "@/types/domain";
import { StatusPill } from "@/components/status-pill";
import { useApplications } from "@/lib/query/hooks";

const filters: Array<{ label: string; statuses: AppStatus[] | null }> = [
  { label: "All", statuses: null },
  { label: "Live", statuses: ["running"] },
  { label: "In progress", statuses: ["building", "undeployed"] },
  { label: "Needs attention", statuses: ["failed"] },
  { label: "Paused & archived", statuses: ["paused", "archived"] },
];

export function AppsIndex({ workspaceSlug }: { workspaceSlug: string }) {
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState(0);
  const { data: applications, isLoading, isError, refetch } = useApplications(workspaceSlug);
  const visibleApplications = useMemo(
    () => applications?.filter((app) => app.name.toLowerCase().includes(search.toLowerCase()) && (filters[filter].statuses?.includes(app.status) ?? true)) ?? [],
    [applications, search, filter],
  );

  // With no apps, search and the header button only repeat the empty state's single action.
  const hasApplications = (applications?.length ?? 0) > 0;

  return (
    <main className="dashboard-page sp-apps">
      <header className="sp-apps-head">
        <span className="sp-eyebrow-dark">Apps</span>
        <h1>Everything you have <em>planted</em>.</h1>
        {hasApplications && <Link className="sp-button sp-button-solid" href={`/workspace/${workspaceSlug}/deploy`}><Plus size={16} /> Deploy app</Link>}
      </header>

      {hasApplications && <div className="apps-toolbar">
        <label className="search-field">
          <Search size={15} />
          <span className="sr-only">Search applications</span>
          <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search apps" />
        </label>
        <div className="sp-chips" role="group" aria-label="Filter by state">
          {filters.map((item, index) => <button type="button" key={item.label} aria-pressed={filter === index} className={filter === index ? "active" : ""} onClick={() => setFilter(index)}>{item.label}</button>)}
        </div>
      </div>}

      {isLoading ? (
        <div className="sp-grid" aria-label="Loading applications">
          {[0, 1, 2, 3].map((item) => <div className="sp-card sp-row-skeleton" key={item} />)}
        </div>
      ) : isError ? (
        <section className="soft-empty"><Boxes size={20} /><h2>Applications could not be loaded</h2><p>Sprout could not reach your applications. You can safely try again.</p><button className="button button-secondary" onClick={() => refetch()}>Try again</button></section>
      ) : applications?.length === 0 ? (
        <section className="apps-empty">
          <h2>No apps yet</h2>
          <p>Deploy one and it will appear here with its own public address.</p>
          <Link className="button button-primary" href={`/workspace/${workspaceSlug}/deploy`}>Deploy your first app <ArrowRight size={16} /></Link>
        </section>
      ) : visibleApplications.length === 0 ? (
        <section className="soft-empty"><Search size={20} /><h2>No applications match</h2><p>Try a different name or filter.</p><button className="button button-secondary" onClick={() => { setSearch(""); setFilter(0); }}>Clear filters</button></section>
      ) : (
        <section className="sp-grid" aria-label="Applications">
          {visibleApplications.map((app) => (
            <Link className={`sp-card sp-card-${app.status}`} href={`/workspace/${workspaceSlug}/apps/${app.id}`} key={app.id}>
              <div className="sp-card-top">
                <span className={`sp-monogram accent-${app.accent}`} aria-hidden="true">{app.name.slice(0, 2).toUpperCase()}</span>
                <StatusPill status={app.status} />
              </div>
              <h2>{app.name}</h2>
              {app.description && <p>{app.description}</p>}
              <div className="sp-card-foot"><span>{app.url}</span><ArrowRight size={15} /></div>
            </Link>
          ))}
        </section>
      )}
    </main>
  );
}
