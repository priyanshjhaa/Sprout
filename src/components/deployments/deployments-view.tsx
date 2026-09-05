"use client";

import { ArrowRight, GitBranch, Rocket, RotateCcw } from "lucide-react";
import Link from "next/link";
import { StatusPill } from "@/components/status-pill";
import { useDeployments } from "@/lib/query/hooks";

export function DeploymentsView({ workspaceSlug, appId }: { workspaceSlug: string; appId: string }) {
  const { data: deployments, isLoading } = useDeployments(workspaceSlug, appId);

  return (
    <section className="app-section">
      <header className="section-heading"><div><h2>Deployments</h2><p>Every version of this application, in order.</p></div><button className="button button-secondary" type="button"><RotateCcw size={14} /> Redeploy</button></header>
      <div className="deployment-list">
        {isLoading ? [0, 1, 2].map((item) => <div className="deployment-row deployment-row-loading" key={item} />) : deployments?.map((deployment, index) => (
          <Link className="deployment-row" href={`/workspace/${workspaceSlug}/apps/${appId}/deployments/${deployment.id}`} key={deployment.id}>
            <span className="deployment-icon"><Rocket size={15} /></span>
            <div className="deployment-main"><div><strong>{deployment.message}</strong>{index === 0 && <span className="current-label">Current</span>}</div><span><GitBranch size={11} /> {deployment.branch} · {deployment.commit}</span></div>
            <StatusPill status={deployment.status} />
            <div className="deployment-time"><strong>{deployment.createdAt}</strong><small>{deployment.duration}</small></div>
            <ArrowRight className="deployment-arrow" size={14} />
          </Link>
        ))}
      </div>
    </section>
  );
}

