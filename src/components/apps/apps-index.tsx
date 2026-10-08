"use client";

import { ArrowRight, Boxes, Plus, Search } from "lucide-react";
import Link from "next/link";
import { useMemo, useState } from "react";
import { StatusPill } from "@/components/status-pill";
import { useApplications } from "@/lib/query/hooks";

export function AppsIndex({ workspaceSlug }: { workspaceSlug: string }) {
  const [search, setSearch] = useState("");
  const { data: applications, isLoading, isError, refetch } = useApplications(workspaceSlug);
  const visibleApplications = useMemo(
    () => applications?.filter((app) => app.name.toLowerCase().includes(search.toLowerCase())) ?? [],
    [applications, search],
  );

  // With no apps, search and the header button only repeat the empty state's single action.
  const hasApplications = (applications?.length ?? 0) > 0;

  return (
    <main className="dashboard-page apps-page">
      <header className="apps-heading">
        <div className="page-heading"><h1>Apps</h1><p>Everything deployed in this workspace.</p></div>
        {hasApplications && <Link className="button button-primary" href={`/workspace/${workspaceSlug}/deploy`}><Plus size={16} /> Deploy app</Link>}
      </header>

      {hasApplications && <div className="apps-toolbar">
        <label className="search-field">
          <Search size={15} />
          <span className="sr-only">Search applications</span>
          <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search apps" />
        </label>
        <span>{applications?.length} {applications?.length === 1 ? "application" : "applications"}</span>
      </div>}

      {isLoading ? (
        <div className="apps-grid" aria-label="Loading applications">
          {[0, 1, 2, 3].map((item) => <div className="app-card app-card-skeleton" key={item} />)}
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
        <section className="soft-empty"><Search size={20} /><h2>No applications match “{search}”</h2><p>Try a different name or clear your search.</p><button className="button button-secondary" onClick={() => setSearch("")}>Clear search</button></section>
      ) : (
        <section className="apps-grid" aria-label="Applications">
          {visibleApplications.map((app) => (
            <Link className="app-card" href={`/workspace/${workspaceSlug}/apps/${app.id}`} key={app.id}>
              <div className="app-card-body">
                <div className="app-card-top">
                  <span className={`app-monogram accent-${app.accent}`} aria-hidden="true">{app.name.slice(0, 2).toUpperCase()}</span>
                  <h2>{app.name}</h2><ArrowRight size={15} />
                </div>
                {app.description && <p>{app.description}</p>}
                <div className="app-card-meta"><StatusPill status={app.status} /><span>Updated {app.updatedAt.toLowerCase()}</span></div>
              </div>
            </Link>
          ))}
        </section>
      )}
    </main>
  );
}
