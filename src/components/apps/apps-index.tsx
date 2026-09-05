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

  return (
    <main className="dashboard-page apps-page">
      <header className="apps-heading">
        <div className="page-heading"><p className="eyebrow">Workspace</p><h1>Apps</h1><p>The small tools your team builds, runs, and shares through Sprout.</p></div>
        <Link className="button button-primary" href={`/workspace/${workspaceSlug}/agent`}><Plus size={16} /> New app</Link>
      </header>

      <div className="apps-toolbar">
        <label className="search-field">
          <Search size={15} />
          <span className="sr-only">Search applications</span>
          <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search apps" />
        </label>
        <span>{applications?.length ?? 0} applications</span>
      </div>

      {isLoading ? (
        <div className="apps-grid" aria-label="Loading applications">
          {[0, 1, 2, 3].map((item) => <div className="app-card app-card-skeleton" key={item} />)}
        </div>
      ) : isError ? (
        <section className="soft-empty"><Boxes size={20} /><h2>Applications could not be loaded</h2><p>The demo data did not respond. You can safely try again.</p><button className="button button-secondary" onClick={() => refetch()}>Try again</button></section>
      ) : visibleApplications.length === 0 ? (
        <section className="soft-empty"><Search size={20} /><h2>No applications match “{search}”</h2><p>Try a different name or clear your search.</p><button className="button button-secondary" onClick={() => setSearch("")}>Clear search</button></section>
      ) : (
        <section className="apps-grid" aria-label="Applications">
          {visibleApplications.map((app) => (
            <Link className="app-card" href={`/workspace/${workspaceSlug}/apps/${app.id}`} key={app.id}>
              <div className={`app-preview accent-${app.accent}`}>
                <span className="preview-title">{app.name}</span>
                <div className="preview-content"><i /><i /><i /></div>
                <span className="preview-glow" />
              </div>
              <div className="app-card-body">
                <div className="app-card-top"><h2>{app.name}</h2><ArrowRight size={15} /></div>
                <p>{app.description}</p>
                <div className="app-card-meta"><StatusPill status={app.status} /><span>{app.updatedAt}</span></div>
                <span className="app-url">{app.url}</span>
              </div>
            </Link>
          ))}
        </section>
      )}
    </main>
  );
}

