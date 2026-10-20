import type { ReactNode } from "react";
import { Plant } from "@/components/plant";
import type { AppStatus } from "@/types/domain";

// An empty page is a bare patch of soil with one clear next step.
export function EmptyState({ plant, title, children, action }: { plant: AppStatus; title: string; children: ReactNode; action?: ReactNode }) {
  return (
    <section className="sp-empty-state">
      <div className="sp-empty-plant"><Plant status={plant} /><span aria-hidden="true" /></div>
      <h2>{title}</h2>
      <p>{children}</p>
      {action}
    </section>
  );
}
