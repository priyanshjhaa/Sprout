import { Box, Check, Download, Globe, Hammer, Rocket, X, type LucideIcon } from "lucide-react";
import type { StageName } from "@/lib/api/simulation-stream";

export type PipelineState = "pending" | "running" | "succeeded" | "failed" | "skipped";
export interface PipelineStage { name: StageName; status: PipelineState }

type StepName = "source" | "build" | "package" | "release" | "live";

// Seven backend stages read as five steps: provisioning and starting the
// release are one "Release" step, and the health check is what makes it "Live".
const steps: Array<{ name: StepName; label: string; hint: string; icon: LucideIcon; from: StageName[] }> = [
  { name: "source", label: "Source", hint: "Your code arrives", icon: Download, from: ["source"] },
  { name: "build", label: "Build", hint: "Built sealed off", icon: Hammer, from: ["build"] },
  { name: "package", label: "Package", hint: "Stored and ready", icon: Box, from: ["package"] },
  { name: "release", label: "Release", hint: "Started in its own container", icon: Rocket, from: ["provision", "release"] },
  { name: "live", label: "Live", hint: "Answers its health check", icon: Globe, from: ["health_check"] },
];

// A step is as far along as its least-finished stage.
function combine(states: PipelineState[]): PipelineState {
  if (states.includes("failed")) return "failed";
  if (states.includes("running")) return "running";
  if (states.every((state) => state === "succeeded")) return "succeeded";
  if (states.every((state) => state === "skipped")) return "skipped";
  return states.some((state) => state === "succeeded") ? "running" : "pending";
}

// Every real deployment travels this path. A simulation stops after Package, so
// it passes simulated to show only the steps it actually performs.
export function Pipeline({ stages, simulated = false, className = "" }: { stages?: PipelineStage[]; simulated?: boolean; className?: string }) {
  const shown = simulated ? steps.slice(0, 3) : steps;
  const stateOf = (from: StageName[]): PipelineState =>
    combine(from.map((name) => stages?.find((stage) => stage.name === name)?.status ?? "pending"));

  return (
    <ol className={`sp-pipeline ${className}`} aria-label="Deployment path">
      {shown.map((step) => {
        const state = stateOf(step.from);
        const Icon = state === "succeeded" ? Check : state === "failed" ? X : step.icon;
        return (
          <li className={`sp-step sp-step-${state}${step.name === "live" ? " sp-step-live" : ""}`} key={step.name}>
            <span className="sp-step-node"><Icon size={17} strokeWidth={2} aria-hidden="true" /></span>
            <strong>{step.label}</strong>
            <small>{state === "pending" ? step.hint : state}</small>
          </li>
        );
      })}
    </ol>
  );
}
