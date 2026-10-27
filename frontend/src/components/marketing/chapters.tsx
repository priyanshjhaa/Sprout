"use client";

import {
  Bot,
  Check,
  FolderUp,
  GitBranch,
  Globe,
  Moon,
  MousePointer2,
  Pause,
  Archive,
  Terminal,
  Zap,
} from "lucide-react";
import { useLoopStep } from "@/components/marketing/use-loop-step";

type Item = { title: string; soon?: boolean };

type ChapterProps = {
  id: string;
  number: string;
  label: string;
  title: string;
  body: string;
  items: Item[];
  active: number;
  flip?: boolean;
  children: React.ReactNode;
};

function Chapter({ id, number, label, title, body, items, active, flip, children }: ChapterProps) {
  return (
    <article className="lp-chapter" id={id} data-flip={flip || undefined} aria-labelledby={`${id}-title`}>
      <div className="lp-chapter-copy" data-reveal>
        <p className="lp-mono">
          {number} — {label}
        </p>
        <h3 id={`${id}-title`}>{title}</h3>
        <p className="lp-lede">{body}</p>
        <ol className="lp-chapter-items">
          {items.map((item, index) => (
            <li key={item.title} className="lp-glass" data-active={index === active || undefined}>
              <span className="lp-mono">
                {number.split(".")[0]}.{index + 1}
              </span>
              <span>{item.title}</span>
              {item.soon && <em>Soon</em>}
            </li>
          ))}
        </ol>
      </div>
      <div className="lp-chapter-mock" data-reveal style={{ "--i": 1 } as React.CSSProperties}>
        {children}
      </div>
    </article>
  );
}

const sources = [
  { icon: FolderUp, name: "Dashboard upload", hint: "drop a folder" },
  { icon: Terminal, name: "CLI", hint: "sprout deploy" },
  { icon: Bot, name: "MCP tool", hint: "from a coding agent" },
  { icon: GitBranch, name: "GitHub repo", hint: "owner/repo@commit" },
];

function DeployChapter() {
  const [ref, step] = useLoopStep<HTMLDivElement>(4, 1800, 0);
  return (
    <Chapter
      id="deploy"
      number="1.0"
      label="Deploy"
      title="Every way in ends at the same deploy API"
      body="Upload a folder, push from your terminal, or let a coding agent ship it. Each one sends a source archive to one typed API, so every app is built and run the same way."
      items={[
        { title: "Upload from the dashboard" },
        { title: "sprout deploy from the CLI" },
        { title: "MCP tool for coding agents", soon: true },
        { title: "Public GitHub repositories", soon: true },
      ]}
      active={step}
    >
      <div className="lp-mock lp-card lp-sources" ref={ref} aria-hidden="true">
        <ul>
          {sources.map((source, index) => {
            const Icon = source.icon;
            return (
              <li key={source.name} className="lp-glass" data-active={index === step || undefined}>
                <Icon size={16} />
                <span>
                  <b>{source.name}</b>
                  <small>{source.hint}</small>
                </span>
              </li>
            );
          })}
        </ul>
        <svg className="lp-sources-lines" viewBox="0 0 100 200" preserveAspectRatio="none">
          {[25, 75, 125, 175].map((y, index) => (
            <path
              key={y}
              d={`M0 ${y} C 50 ${y}, 50 100, 100 100`}
              data-active={index === step || undefined}
              vectorEffect="non-scaling-stroke"
            />
          ))}
        </svg>
        <div className="lp-sources-api" key={step}>
          <span className="lp-mono">POST</span>
          <b>/deployments</b>
          <small>source.tar.gz</small>
        </div>
      </div>
    </Chapter>
  );
}

const bars = [38, 52, 44, 70, 62, 80, 58, 66, 74, 50, 64, 72];

const operateStates = [
  { label: "Live", note: "Serving visitors" },
  { label: "Live", note: "No visits for a while" },
  { label: "Asleep", note: "Released its CPU and memory" },
  { label: "Waking", note: "A visitor opened the URL" },
  { label: "Live", note: "Answered /health, serving again" },
];

function OperateChapter() {
  const [ref, step] = useLoopStep<HTMLDivElement>(5, 1700);
  const state = operateStates[step];
  const itemForStep = [0, 2, 2, 2, 0][step];
  return (
    <Chapter
      id="operate"
      number="2.0"
      label="Operate"
      title="Healthy while it's used, asleep when it isn't"
      body="Sprout watches every app's health and keeps its deployments and build logs one click away. Apps nobody is visiting go to sleep and wake on the next request, so a garden of small apps stays cheap."
      items={[
        { title: "Live status and health checks" },
        { title: "Deployments and build logs" },
        { title: "Sleep when idle, wake on a visit" },
        { title: "Pause and resume" },
      ]}
      active={itemForStep}
      flip
    >
      <div className="lp-mock lp-card lp-operate" ref={ref} data-step={step} aria-hidden="true">
        <div className="lp-operate-head">
          <div>
            <p className="lp-mono">hiring-board</p>
            <b>{state.label}</b>
          </div>
          <span className="lp-operate-icon">
            {step === 2 ? <Moon size={18} /> : step === 3 ? <Zap size={18} /> : <Check size={18} />}
          </span>
        </div>
        <div className="lp-operate-bars">
          {bars.map((height, index) => (
            <i key={index} style={{ "--h": `${height}%`, "--d": `${index * 40}ms` } as React.CSSProperties} />
          ))}
          <span className="lp-operate-zs">
            <i>z</i>
            <i>z</i>
            <i>z</i>
          </span>
        </div>
        <p className="lp-operate-note">{state.note}</p>
        <div className="lp-operate-url">
          <Globe size={14} />
          <span>hiring-board.acme.sprout.run</span>
          <MousePointer2 className="lp-operate-cursor" size={18} />
        </div>
      </div>
    </Chapter>
  );
}

const members = [
  { initials: "PJ", name: "Priya", role: "Owner" },
  { initials: "AC", name: "Alex", role: "Editor" },
  { initials: "BH", name: "Bea", role: "Viewer" },
];

const lifecycle = [
  { icon: Globe, label: "Live" },
  { icon: Pause, label: "Paused" },
  { icon: Archive, label: "Archived" },
];

function ShareChapter() {
  const [ref, step] = useLoopStep<HTMLDivElement>(5, 1600);
  const shownMembers = Math.min(step + 1, members.length);
  const lifecycleStep = Math.max(0, step - 2);
  return (
    <Chapter
      id="share"
      number="3.0"
      label="Share"
      title="Anyone can open it. Your team decides what happens to it."
      body="Every live app has a public URL to send to whoever needs it. Workspace roles decide who can deploy, change, pause, or archive it, and that control ends when someone leaves."
      items={[
        { title: "A public URL for every live app" },
        { title: "Owner, editor, and viewer roles" },
        { title: "Invitations by link" },
        { title: "Pause, archive, and restore" },
      ]}
      active={step < 3 ? Math.min(step, 2) : 3}
    >
      <div className="lp-mock lp-card lp-share" ref={ref} aria-hidden="true">
        <p className="lp-mono">Access · invoice-approval</p>
        <ul className="lp-members">
          {members.slice(0, shownMembers).map((member) => (
            <li key={member.initials}>
              <i>{member.initials}</i>
              <span>{member.name}</span>
              <em data-role={member.role.toLowerCase()}>{member.role}</em>
            </li>
          ))}
        </ul>
        <p className="lp-mono">Lifecycle</p>
        <div className="lp-lifecycle" style={{ "--l": lifecycleStep } as React.CSSProperties}>
          <span className="lp-lifecycle-track" />
          {lifecycle.map((stage, index) => {
            const Icon = stage.icon;
            return (
              <span key={stage.label} data-reached={index <= lifecycleStep || undefined}>
                <Icon size={14} />
                {stage.label}
              </span>
            );
          })}
        </div>
      </div>
    </Chapter>
  );
}

export function Chapters() {
  return (
    <section className="lp-chapters" aria-labelledby="chapters-title">
      <div className="lp-container">
        <header className="lp-chapters-head" data-reveal>
          <p className="lp-mono">What Sprout does</p>
          <h2 id="chapters-title" className="lp-heading">
            Keep writing the code. Let Sprout look after the apps.
          </h2>
          <p className="lp-lede">
            Start with code that works on your machine. Sprout builds it, runs it, and keeps it reachable, while you
            stay in control of what goes live.
          </p>
        </header>
        <DeployChapter />
        <OperateChapter />
        <ShareChapter />
      </div>
    </section>
  );
}
