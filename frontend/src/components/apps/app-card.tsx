import Link from "next/link";
import { Plant } from "@/components/plant";
import type { AppStatus, Application } from "@/types/domain";

export const statusLabels: Record<AppStatus, string> = {
  running: "Live",
  building: "Building",
  failed: "Needs attention",
  undeployed: "Ready",
  paused: "Paused",
  archived: "Archived",
};

// An app as a card: a plant for its state, then its name, address, and status.
export function AppCard({ app, base, index }: { app: Application; base: string; index: number }) {
  return (
    <li style={{ "--i": index } as React.CSSProperties}>
      <Link className="dash-card" href={`${base}/apps/${app.id}`} data-status={app.status}>
        <span className="dash-card-art" aria-hidden="true">
          <Plant status={app.status} />
        </span>
        <span className="dash-card-body">
          <b>{app.name}</b>
          <small className="dash-card-url">{app.url}</small>
          {app.description && <span className="dash-card-description">{app.description}</span>}
        </span>
        <span className="dash-card-foot">
          <span className="dash-card-status">
            <i data-status={app.status} aria-hidden="true" /> {statusLabels[app.status]}
          </span>
          <span className="lp-mono">{app.updatedAt}</span>
        </span>
      </Link>
    </li>
  );
}
