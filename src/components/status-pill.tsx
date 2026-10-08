import type { AppStatus } from "@/types/domain";
import type { Simulation } from "@/lib/api/simulation-stream";

const labels: Record<AppStatus | Simulation["status"], string> = {
  undeployed: "Not deployed",
  queued: "Queued",
  succeeded: "Succeeded",
  cancelled: "Cancelled",
  running: "Running",
  building: "Building",
  failed: "Needs attention",
  paused: "Paused",
  archived: "Archived",
};

export function StatusPill({ status }: { status: AppStatus | Simulation["status"] }) {
  return (
    <span className={`status-pill status-${status}`}>
      <span className="status-dot" aria-hidden="true" />
      {labels[status]}
    </span>
  );
}
