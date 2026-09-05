"use client";

import { Check, Circle, Clock3, Copy, RotateCcw } from "lucide-react";
import Link from "next/link";
import { StatusPill } from "@/components/status-pill";
import { useDeployments } from "@/lib/query/hooks";

const stages = ["Queued", "Building", "Starting", "Health check", "Live"];
const buildLogs = [
  "10:40:11  Cloning repository at a81f29c",
  "10:40:14  Detected Next.js application",
  "10:40:15  Installing dependencies",
  "10:40:33  Creating optimized production build",
  "10:40:47  Container image ready",
  "10:40:54  Health check passed on port 3000",
];

export function DeploymentDetail({ workspaceSlug, appId, deploymentId }: { workspaceSlug: string; appId: string; deploymentId: string }) {
  const { data: deployments, isLoading } = useDeployments(workspaceSlug, appId);
  const deployment = deployments?.find((item) => item.id === deploymentId);

  if (isLoading) return <div className="deployment-detail-loading" />;
  if (!deployment) return <section className="soft-empty"><Circle size={20} /><h2>Deployment not found</h2><p>This deployment is not part of the demo history.</p></section>;
  const completedStages = deployment.status === "failed" ? 2 : stages.length;

  return (
    <section className="app-section deployment-detail">
      <Link className="section-back" href={`/workspace/${workspaceSlug}/apps/${appId}/deployments`}>← Deployments</Link>
      <header className="section-heading"><div><div className="deployment-title"><h2>{deployment.message}</h2><StatusPill status={deployment.status} /></div><p>{deployment.branch} · {deployment.commit} · {deployment.createdAt}</p></div><button className="button button-secondary" type="button"><RotateCcw size={14} /> Retry deployment</button></header>

      <ol className="deployment-stages">
        {stages.map((stage, index) => (
          <li className={index < completedStages ? "complete" : index === completedStages ? "failed" : ""} key={stage}>
            <span>{index < completedStages ? <Check size={12} /> : <Circle size={10} />}</span><strong>{stage}</strong>{index < completedStages && <small>{index === completedStages - 1 ? deployment.duration : `${index * 9 + 3}s`}</small>}
          </li>
        ))}
      </ol>

      <div className="build-log-panel">
        <header><div><span>Build output</span><small>deployment/{deployment.id}</small></div><button type="button" aria-label="Copy build logs"><Copy size={14} /></button></header>
        <pre>{buildLogs.slice(0, deployment.status === "failed" ? 4 : undefined).map((line) => <code key={line}>{line}</code>)}{deployment.status === "failed" && <code className="log-error">10:40:36  ERROR Build command exited with code 1</code>}</pre>
        <footer><Clock3 size={12} /> Build finished in {deployment.duration}</footer>
      </div>
    </section>
  );
}

