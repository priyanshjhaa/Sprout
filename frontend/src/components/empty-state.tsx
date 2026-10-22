import type { ReactNode } from "react";
import { Plant } from "@/components/plant";
import type { AppStatus } from "@/types/domain";

// An empty page is a bare patch of soil with one clear next step.
export function EmptyState({ plant, title, children, action }: { plant: AppStatus; title: string; children: ReactNode; action?: ReactNode }) {
  return (
    <section className="dash-empty">
      <Plant status={plant} />
      <h3>{title}</h3>
      <p>{children}</p>
      {action}
    </section>
  );
}
