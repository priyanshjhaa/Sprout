import type { AppStatus, DeploymentStatus } from "@/types/domain";

const labels: Record<AppStatus | DeploymentStatus, string> = {
  running: "Running",
  building: "Building",
  failed: "Needs attention",
  paused: "Paused",
  live: "Live",
};

export function StatusPill({ status }: { status: AppStatus | DeploymentStatus }) {
  return (
    <span className={`status-pill status-${status}`}>
      <span className="status-dot" aria-hidden="true" />
      {labels[status]}
    </span>
  );
}

