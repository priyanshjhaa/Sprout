import { Box, Check, Download, Globe, Hammer, X, type LucideIcon } from "lucide-react";

export type PipelineState = "pending" | "running" | "succeeded" | "failed" | "skipped";
export interface PipelineStage { name: "source" | "build" | "package"; status: PipelineState }

const steps: Array<{ name: PipelineStage["name"] | "live"; label: string; hint: string; icon: LucideIcon }> = [
  { name: "source", label: "Source", hint: "Your code arrives", icon: Download },
  { name: "build", label: "Build", hint: "Built sealed off", icon: Hammer },
  { name: "package", label: "Package", hint: "Stored and ready", icon: Box },
  { name: "live", label: "Live", hint: "Healthy public URL", icon: Globe },
];

// Every deployment travels this path. Live is shown as the destination, not a
// state: the runtime that serves it is not part of the current deploy API.
export function Pipeline({ stages, className = "" }: { stages?: PipelineStage[]; className?: string }) {
  const stateOf = (name: string): PipelineState => stages?.find((stage) => stage.name === name)?.status ?? "pending";

  return (
    <ol className={`sp-pipeline ${className}`} aria-label="Deployment path">
      {steps.map((step) => {
        const live = step.name === "live";
        const state = live ? "pending" : stateOf(step.name);
        const Icon = state === "succeeded" ? Check : state === "failed" ? X : step.icon;
        return (
          <li className={`sp-step sp-step-${state}${live ? " sp-step-live" : ""}`} key={step.name}>
            <span className="sp-step-node"><Icon size={17} strokeWidth={2} aria-hidden="true" /></span>
            <strong>{step.label}</strong>
            <small>{live ? "Runtime comes next" : state === "pending" ? step.hint : state}</small>
          </li>
        );
      })}
    </ol>
  );
}
