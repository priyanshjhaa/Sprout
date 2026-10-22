"use client";

import { Check, Copy, ExternalLink } from "lucide-react";
import { useEffect, useState } from "react";

// An app's address with one-click copy, and Open once it is actually live.
export function CopyAddress({ url, live }: { url: string; live: boolean }) {
  const [state, setState] = useState<"idle" | "copied" | "failed">("idle");
  useEffect(() => {
    if (state === "idle") return;
    const timer = window.setTimeout(() => setState("idle"), 1800);
    return () => window.clearTimeout(timer);
  }, [state]);

  async function copy() {
    try { await navigator.clipboard.writeText(url); setState("copied"); }
    catch { setState("failed"); }
  }

  return (
    <div className="dash-address">
      <span className="dash-address-label">{live ? "Live at" : "Reserved address"}</span>
      <code>{url}</code>
      <button type="button" className="dash-address-button" onClick={copy} aria-label={`Copy ${url}`}>
        {state === "copied" ? <Check size={14} /> : <Copy size={14} />}
        <span>{state === "copied" ? "Copied" : state === "failed" ? "Copy failed" : "Copy"}</span>
      </button>
      {live && <a className="dash-address-button" href={`https://${url}`} target="_blank" rel="noreferrer"><ExternalLink size={14} /><span>Open</span></a>}
      <span className="sr-only" role="status">{state === "copied" ? "Address copied" : ""}</span>
    </div>
  );
}
