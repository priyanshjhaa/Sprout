"use client";

import { ArrowUp, Check, ChevronRight, ExternalLink, LoaderCircle, Sparkles } from "lucide-react";
import Link from "next/link";
import { FormEvent, useState } from "react";
import { StatusPill } from "@/components/status-pill";
import { useAgentEvents } from "@/lib/query/hooks";

const suggestions = [
  "Build an invoice approval tool",
  "Make a lightweight hiring tracker",
  "Create a customer research library",
];

export function AgentWorkspace({ workspaceSlug }: { workspaceSlug: string }) {
  const [prompt, setPrompt] = useState("");
  const [request, setRequest] = useState<string | null>(null);
  const { data: events, isLoading } = useAgentEvents(workspaceSlug);

  function submit(event: FormEvent) {
    event.preventDefault();
    const trimmed = prompt.trim();
    if (!trimmed) return;
    setRequest(trimmed);
    setPrompt("");
  }

  return (
    <main className="dashboard-page agent-page">
      <header className="page-heading agent-heading">
        <div><p className="eyebrow">Agent workspace</p><h1>What should we grow?</h1></div>
        <span className="agent-availability"><i /> Ready</span>
      </header>

      <div className="agent-layout">
        <section className="agent-conversation" aria-label="Agent conversation">
          {!request ? (
            <div className="agent-welcome">
              <span className="agent-spark"><Sparkles size={19} /></span>
              <h2>Describe the useful thing.</h2>
              <p>Start with the workflow, not the infrastructure. Sprout will keep the path to a shared application clear.</p>
              <div className="prompt-suggestions">
                {suggestions.map((suggestion) => (
                  <button key={suggestion} type="button" onClick={() => setPrompt(suggestion)}>
                    {suggestion}<ChevronRight size={14} />
                  </button>
                ))}
              </div>
            </div>
          ) : (
            <div className="agent-thread">
              <article className="user-message"><span>PJ</span><p>{request}</p></article>
              <article className="agent-message">
                <span className="agent-spark"><Sparkles size={15} /></span>
                <div><strong>Your app is ready.</strong><p>I created a focused invoice workflow, connected its data, and checked the running application.</p></div>
              </article>
              <article className="agent-app-artifact">
                <div className="artifact-top"><span className="artifact-mark">IA</span><div><strong>Invoice approvals</strong><small>invoice-acme.sprout.run</small></div><StatusPill status="running" /></div>
                <div className="artifact-preview"><span>Invoices</span><i /><i /><i /></div>
                <div className="artifact-actions">
                  <Link href={`/workspace/${workspaceSlug}/apps/invoice-approvals`}>View application</Link>
                  <a href="https://invoice-acme.sprout.run" target="_blank" rel="noreferrer">Open <ExternalLink size={13} /></a>
                </div>
              </article>
            </div>
          )}

          <form className="agent-composer" onSubmit={submit}>
            <label className="sr-only" htmlFor="agent-prompt">Describe an application</label>
            <textarea
              id="agent-prompt"
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              placeholder="Build a small tool for…"
              rows={3}
            />
            <div className="composer-footer">
              <span>Demo mode · no code will run</span>
              <button type="submit" aria-label="Send prompt" disabled={!prompt.trim()}><ArrowUp size={17} /></button>
            </div>
          </form>
        </section>

        <aside className="agent-activity" aria-label="Agent activity">
          <div className="activity-header"><div><span>Activity</span><small>User-relevant actions</small></div>{request && <StatusPill status="live" />}</div>
          {!request ? (
            <div className="activity-idle"><Sparkles size={18} /><p>Build activity will appear here once you describe an application.</p></div>
          ) : isLoading ? (
            <div className="activity-idle"><LoaderCircle className="spin" size={18} /><p>Loading activity…</p></div>
          ) : (
            <ol className="activity-list">
              {events?.map((item) => (
                <li key={item.id}><span><Check size={12} /></span><div><strong>{item.label}</strong><small>{item.detail}</small></div></li>
              ))}
            </ol>
          )}
          <div className="activity-note"><p>Sprout shows outcomes and errors here—not hidden reasoning or noisy command output.</p></div>
        </aside>
      </div>
    </main>
  );
}

