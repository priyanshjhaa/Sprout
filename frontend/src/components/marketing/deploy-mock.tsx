"use client";

import { Check, FolderUp, Globe, LockKeyhole, Play, Stethoscope } from "lucide-react";
import { useLoopStep } from "@/components/marketing/use-loop-step";

const stages = [
  { icon: FolderUp, title: "Upload source", detail: "folder · 14 files · 38 KB" },
  { icon: LockKeyhole, title: "Build, sealed off", detail: "no network · 1 CPU · 1 GiB · 2 min limit" },
  { icon: Play, title: "Start", detail: "non-root · read-only · 0.5 CPU · 256 MiB" },
  { icon: Stethoscope, title: "Health check", detail: "GET /health → 200 OK" },
];

const logs = [
  "received archive, 14 files",
  "npm ci --offline",
  "npm --ignore-scripts run build",
  "artifact saved · sha256:9f2c…e1",
  "container started on :8080",
  "/health answered in 41 ms",
  "routing invoice-approval.acme.sprout.run",
];

const apps = [
  { name: "invoice-approval", state: "deploying" },
  { name: "hiring-board", state: "live" },
  { name: "research-notes", state: "asleep" },
];

// Step 0..3 runs each stage; step 4 is live.
export function DeployMock() {
  const [ref, step] = useLoopStep<HTMLDivElement>(5, 1500);
  const visibleLogs = Math.min(logs.length, Math.round((step / 4) * logs.length) + 1);

  return (
    <div className="lp-window lp-card" ref={ref} aria-hidden="true" data-step={step}>
      <div className="lp-window-bar">
        <i />
        <i />
        <i />
        <span>sprout.run/acme/deploy</span>
      </div>
      <div className="lp-window-body">
        <aside className="lp-window-side">
          <p className="lp-mono">Workspace</p>
          <ul className="lp-window-nav">
            <li data-active>Deploy</li>
            <li>Apps</li>
            <li>Team</li>
          </ul>
          <p className="lp-mono">Apps</p>
          <ul className="lp-window-apps">
            {apps.map((app) => (
              <li key={app.name} data-state={app.name === "invoice-approval" && step === 4 ? "live" : app.state}>
                <i />
                {app.name}
              </li>
            ))}
          </ul>
        </aside>

        <div className="lp-window-main">
          <div className="lp-deploy-head">
            <div>
              <p className="lp-mono">Deployment #12</p>
              <h3>invoice-approval</h3>
            </div>
            <span className="lp-deploy-badge" data-live={step === 4 || undefined}>
              {step === 4 ? "Live" : "Deploying"}
            </span>
          </div>

          <ol className="lp-stages">
            {stages.map((stage, index) => {
              const state = index < step ? "done" : index === step ? "running" : "pending";
              const Icon = stage.icon;
              return (
                <li key={stage.title} className="lp-glass" data-state={state}>
                  <span className="lp-stage-icon">
                    {state === "done" ? <Check size={14} /> : <Icon size={14} />}
                  </span>
                  <span className="lp-stage-text">
                    <b>{stage.title}</b>
                    <small>{stage.detail}</small>
                  </span>
                  <span className="lp-stage-bar" />
                </li>
              );
            })}
          </ol>

          <div className="lp-logs">
            {logs.slice(0, visibleLogs).map((line) => (
              <p key={line}>
                <span>›</span> {line}
              </p>
            ))}
          </div>

          <div className="lp-url" data-shown={step === 4 || undefined}>
            <Globe size={15} />
            <span>invoice-approval.acme.sprout.run</span>
            <b>Open</b>
          </div>
        </div>
      </div>
    </div>
  );
}
