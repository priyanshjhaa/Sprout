"use client";

import { Database, Eye, EyeOff, HardDrive, KeyRound, Plus, Trash2, X } from "lucide-react";
import { FormEvent, useMemo, useState } from "react";
import { useEnvironment } from "@/lib/query/hooks";
import type { EnvironmentVariable } from "@/types/domain";

export function EnvironmentView({ workspaceSlug, appId }: { workspaceSlug: string; appId: string }) {
  const { data: variables, isLoading } = useEnvironment(workspaceSlug, appId);
  const [added, setAdded] = useState<EnvironmentVariable[]>([]);
  const [removed, setRemoved] = useState<string[]>([]);
  const [revealed, setRevealed] = useState<string[]>([]);
  const [adding, setAdding] = useState(false);
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const visibleVariables = useMemo(() => [...(variables ?? []), ...added].filter((item) => !removed.includes(item.key)), [added, removed, variables]);

  function addVariable(event: FormEvent) {
    event.preventDefault();
    const normalizedKey = key.trim().toUpperCase().replace(/\s+/g, "_");
    if (!normalizedKey || !value.trim()) return;
    setAdded((items) => [...items.filter((item) => item.key !== normalizedKey), { key: normalizedKey, value: value.trim(), managed: false, secret: true }]);
    setRemoved((items) => items.filter((item) => item !== normalizedKey));
    setKey("");
    setValue("");
    setAdding(false);
  }

  return (
    <section className="app-section environment-view">
      <header className="section-heading"><div><h2>Environment</h2><p>Configuration and resources available to this application.</p></div><button className="button button-secondary" type="button" onClick={() => setAdding(true)}><Plus size={14} /> Add variable</button></header>

      {adding && <form className="inline-variable-form" onSubmit={addVariable}>
        <label><span>Key</span><input autoFocus value={key} onChange={(event) => setKey(event.target.value)} placeholder="SERVICE_API_KEY" /></label>
        <label><span>Value</span><input value={value} onChange={(event) => setValue(event.target.value)} placeholder="Stored as a secret" /></label>
        <button className="button button-primary" type="submit">Save variable</button>
        <button className="icon-button" type="button" aria-label="Cancel adding variable" onClick={() => setAdding(false)}><X size={15} /></button>
      </form>}

      <div className="environment-grid">
        <section className="configuration-panel variables-panel">
          <header><div><span className="panel-icon"><KeyRound size={15} /></span><div><h3>Variables</h3><p>Available to the application at runtime.</p></div></div><span>{visibleVariables.length}</span></header>
          <div className="variable-list">
            {isLoading ? <div className="variable-loading">Loading environment…</div> : visibleVariables.map((variable) => {
              const canReveal = variable.secret && !variable.managed;
              const isRevealed = revealed.includes(variable.key);
              return <div className="variable-row" key={variable.key}>
                <div><strong>{variable.key}</strong><small>{variable.managed ? "Managed by Sprout" : "User variable"}</small></div>
                <code>{variable.secret && !isRevealed ? "••••••••••••••" : variable.value}</code>
                {canReveal && <button type="button" aria-label={`${isRevealed ? "Hide" : "Reveal"} ${variable.key}`} onClick={() => setRevealed((items) => isRevealed ? items.filter((item) => item !== variable.key) : [...items, variable.key])}>{isRevealed ? <EyeOff size={14} /> : <Eye size={14} />}</button>}
                {!variable.managed && <button className="danger-icon" type="button" aria-label={`Remove ${variable.key}`} onClick={() => setRemoved((items) => [...items, variable.key])}><Trash2 size={14} /></button>}
              </div>;
            })}
          </div>
          <footer>Variable changes require a new deployment before they take effect.</footer>
        </section>

        <section className="configuration-panel resources-panel">
          <header><div><span className="panel-icon"><Database size={15} /></span><div><h3>Resources</h3><p>Data services attached to this app.</p></div></div></header>
          <div className="resource-detail"><span><Database size={16} /></span><div><strong>PostgreSQL</strong><small>shared-small · healthy</small></div><i /></div>
          <div className="resource-detail"><span><HardDrive size={16} /></span><div><strong>Object storage</strong><small>No bucket connected</small></div><button type="button">Connect</button></div>
        </section>
      </div>
    </section>
  );
}

