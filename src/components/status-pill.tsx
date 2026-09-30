import type { AppStatus, DeploymentStatus } from "@/types/domain";
import type { Simulation } from "@/lib/api/simulation-stream";

const labels: Record<AppStatus | DeploymentStatus | Simulation["status"], string> = {
  queued: "Queued",
  succeeded: "Succeeded",
  cancelled: "Cancelled",
  running: "Running",
  building: "Building",
  failed: "Needs attention",
  paused: "Paused",
  archived: "Archived",
  live: "Live",
};

export function StatusPill({ status }: { status: AppStatus | DeploymentStatus | Simulation["status"] }) {
  return (
    <span className={`status-pill status-${status}`}>
      <span className="status-dot" aria-hidden="true" />
      {labels[status]}
    </span>
  );
}
