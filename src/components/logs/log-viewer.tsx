"use client";

import { Copy, Download, Pause, Play, Search } from "lucide-react";
import { useMemo, useState } from "react";
import { useLogs } from "@/lib/query/hooks";

export function LogViewer({ workspaceSlug, appId }: { workspaceSlug: string; appId: string }) {
  const { data: logs, isLoading } = useLogs(workspaceSlug, appId);
  const [query, setQuery] = useState("");
  const [level, setLevel] = useState("all");
  const [paused, setPaused] = useState(false);
  const visibleLogs = useMemo(() => logs?.filter((log) => {
    const matchesLevel = level === "all" || log.level === level;
    const matchesQuery = log.message.toLowerCase().includes(query.toLowerCase());
    return matchesLevel && matchesQuery;
  }) ?? [], [level, logs, query]);

  return (
    <section className="app-section">
      <header className="section-heading"><div><h2>Runtime logs</h2><p>What the running application is doing right now.</p></div><span className={`stream-state${paused ? " paused" : ""}`}><i />{paused ? "Paused" : "Streaming"}</span></header>
      <div className="logs-panel">
        <div className="logs-toolbar">
          <label className="log-search"><Search size={13} /><span className="sr-only">Search logs</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search logs" /></label>
          <label><span className="sr-only">Filter by level</span><select value={level} onChange={(event) => setLevel(event.target.value)}><option value="all">All levels</option><option value="info">Info</option><option value="warn">Warning</option><option value="error">Error</option></select></label>
          <select aria-label="Time range" defaultValue="30m"><option value="30m">Last 30 minutes</option><option value="1h">Last hour</option><option value="24h">Last 24 hours</option></select>
          <span className="toolbar-spacer" />
          <button type="button" onClick={() => setPaused((value) => !value)} aria-label={paused ? "Resume log stream" : "Pause log stream"}>{paused ? <Play size={14} /> : <Pause size={14} />}</button>
          <button type="button" aria-label="Copy logs"><Copy size={14} /></button>
          <button type="button" aria-label="Download logs"><Download size={14} /></button>
        </div>
        <div className="log-lines" aria-live={paused ? "off" : "polite"}>
          {isLoading ? <span className="logs-loading">Connecting to the demo stream…</span> : visibleLogs.length ? visibleLogs.map((log) => (
            <div className={`log-line log-${log.level}`} key={log.id}><time>{log.timestamp}</time><span className="log-level">{log.level}</span><span className="log-method">{log.method ?? "SYS"}</span><code>{log.message}</code></div>
          )) : <div className="no-log-results">No log entries match the current filters.</div>}
        </div>
        <footer className="logs-footer"><span>{visibleLogs.length} events</span><span>Demo stream · us-east</span></footer>
      </div>
    </section>
  );
}

