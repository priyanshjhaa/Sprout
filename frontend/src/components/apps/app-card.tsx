"use client";

import { Check, Copy, ExternalLink } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
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
// The name's link covers the card; live apps add Open and Copy on top of it.
export function AppCard({ app, base, index }: { app: Application; base: string; index: number }) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    if (!app.liveUrl) return;
    try {
      await navigator.clipboard.writeText(app.liveUrl);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      // Clipboard access can be refused; the address stays visible on the card.
    }
  }

  return (
    <li style={{ "--i": index } as React.CSSProperties}>
      <article className="dash-card" data-status={app.status}>
        <span className="dash-card-art" aria-hidden="true">
          <Plant status={app.status} />
          {app.status === "building" && <span className="dash-card-progress" />}
        </span>
        <span className="dash-card-body">
          <Link className="dash-card-link" href={`${base}/apps/${app.id}`}>
            {app.name}
          </Link>
          <small className="dash-card-url">{app.url}</small>
          {app.description && <span className="dash-card-description">{app.description}</span>}
        </span>
        <span className="dash-card-foot">
          <span className="dash-card-status">
            <i data-status={app.status} aria-hidden="true" /> {statusLabels[app.status]}
          </span>
          <span className="lp-mono">{app.updatedAt}</span>
        </span>
        {app.liveUrl && (
          <span className="dash-card-actions">
            <a href={app.liveUrl} target="_blank" rel="noreferrer" aria-label={`Open ${app.name}`}>
              <ExternalLink size={13} aria-hidden="true" /> Open
            </a>
            <button type="button" onClick={copy} aria-label={copied ? "Address copied" : `Copy ${app.name}'s address`}>
              {copied ? <Check size={13} aria-hidden="true" /> : <Copy size={13} aria-hidden="true" />}
            </button>
          </span>
        )}
      </article>
    </li>
  );
}
