"use client";

import { ArrowRight, Boxes, Plus, Search } from "lucide-react";
import Link from "next/link";
import { useMemo, useState } from "react";
import type { AppStatus } from "@/types/domain";
import { AppCard } from "@/components/apps/app-card";
import { PageHeader } from "@/components/dashboard/page-header";
import { EmptyState } from "@/components/empty-state";
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
  const { data: applications, isPending: isLoading, isError, refetch } = useApplications(workspaceSlug);
  const visibleApplications = useMemo(
    () => applications?.filter((app) => app.name.toLowerCase().includes(search.toLowerCase()) && (filters[filter].statuses?.includes(app.status) ?? true)) ?? [],
    [applications, search, filter],
  );
  const base = `/workspace/${workspaceSlug}`;

  // With no apps, search and the header button only repeat the empty state's single action.
  const hasApplications = (applications?.length ?? 0) > 0;

  return (
    <main className="dashboard-page dash-stack">
      <PageHeader
        eyebrow="Apps"
        title="Everything you have planted"
        description="Each app keeps its own public address, deployments, and access."
        actions={
          hasApplications && (
            <Link className="lp-btn lp-btn--dark lp-btn--small" href={`${base}/deploy`}>
              <Plus size={15} aria-hidden="true" /> Deploy app
            </Link>
          )
        }
      />

      {hasApplications && (
        <div className="dash-toolbar">
          <label className="dash-search">
            <Search size={15} aria-hidden="true" />
            <span className="sr-only">Search applications</span>
            <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search apps" />
          </label>
          <div className="dash-tabs" role="group" aria-label="Filter by state">
            {filters.map((item, index) => (
              <button type="button" key={item.label} aria-pressed={filter === index} onClick={() => setFilter(index)}>
                {item.label}
              </button>
            ))}
          </div>
        </div>
      )}

      {isLoading ? (
        <ul className="dash-cards" aria-label="Loading applications">
          {[0, 1, 2, 3].map((item) => <li className="dash-card dash-card-skeleton" key={item} />)}
        </ul>
      ) : isError ? (
        <section className="dash-empty">
          <Boxes size={20} aria-hidden="true" />
          <h3>Applications could not be loaded</h3>
          <p>Sprout could not reach your applications. You can safely try again.</p>
          <button className="lp-btn lp-btn--light lp-btn--small" type="button" onClick={() => refetch()}>Try again</button>
        </section>
      ) : applications?.length === 0 ? (
        <EmptyState plant="undeployed" title="No apps yet" action={<Link className="lp-btn lp-btn--dark lp-btn--small" href={`${base}/deploy`}>Deploy your first app <ArrowRight size={15} aria-hidden="true" /></Link>}>
          Deploy one and it will appear here with its own public address.
        </EmptyState>
      ) : visibleApplications.length === 0 ? (
        <EmptyState plant="archived" title="Nothing matches" action={<button className="lp-btn lp-btn--light lp-btn--small" type="button" onClick={() => { setSearch(""); setFilter(0); }}>Clear filters</button>}>
          No app fits that name or state. Try another search or filter.
        </EmptyState>
      ) : (
        <ul className="dash-cards" aria-label="Applications">
          {visibleApplications.map((app, index) => <AppCard key={app.id} app={app} base={base} index={index} />)}
        </ul>
      )}
    </main>
  );
}
