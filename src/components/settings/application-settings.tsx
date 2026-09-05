"use client";

import { GitBranch, Globe2, Save, Server, Trash2, TriangleAlert, X } from "lucide-react";
import { useState } from "react";

export function ApplicationSettings() {
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [confirmation, setConfirmation] = useState("");

  return (
    <section className="app-section settings-view">
      <header className="section-heading"><div><h2>Settings</h2><p>Infrequent application configuration, kept in one place.</p></div><button className="button button-primary" type="button"><Save size={14} /> Save changes</button></header>

      <section className="settings-panel">
        <div className="settings-panel-heading"><span className="panel-icon"><Server size={15} /></span><div><h3>General</h3><p>Application identity and runtime defaults.</p></div></div>
        <div className="settings-fields">
          <label><span>Application name</span><input defaultValue="Invoice approvals" /></label>
          <label><span>Runtime port</span><input defaultValue="3000" inputMode="numeric" /></label>
        </div>
      </section>

      <section className="settings-panel">
        <div className="settings-panel-heading"><span className="panel-icon"><GitBranch size={15} /></span><div><h3>Source</h3><p>The repository and branch used for new deployments.</p></div></div>
        <div className="connected-setting"><div><strong>sprout-demo/invoice-approvals</strong><small>Deploy from main · GitHub connection mocked</small></div><button className="button button-secondary" type="button">Change repository</button></div>
      </section>

      <section className="settings-panel">
        <div className="settings-panel-heading"><span className="panel-icon"><Globe2 size={15} /></span><div><h3>Domains</h3><p>The default address is already secured by Sprout.</p></div></div>
        <div className="connected-setting"><div><strong>invoice-acme.sprout.run</strong><small>Default domain · HTTPS active</small></div><button className="button button-secondary" type="button">Add custom domain</button></div>
      </section>

      <section className="settings-panel danger-panel">
        <div className="settings-panel-heading"><span className="panel-icon"><TriangleAlert size={15} /></span><div><h3>Delete application</h3><p>Remove the application and its deployment history from this workspace.</p></div></div>
        <button className="button danger-button" type="button" onClick={() => setDeleteOpen(true)}><Trash2 size={14} /> Delete application</button>
      </section>

      {deleteOpen && <div className="dialog-backdrop" role="presentation">
        <section className="confirmation-dialog" role="alertdialog" aria-modal="true" aria-labelledby="delete-title" aria-describedby="delete-description">
          <button className="dialog-close" type="button" onClick={() => setDeleteOpen(false)} aria-label="Close confirmation"><X size={16} /></button>
          <span className="dialog-danger-icon"><Trash2 size={18} /></span>
          <h2 id="delete-title">Delete Invoice approvals?</h2>
          <p id="delete-description">This frontend preview will not delete data. In the real product this action will remove deployments and disconnect resources.</p>
          <label><span>Type <strong>invoice-approvals</strong> to confirm</span><input autoFocus value={confirmation} onChange={(event) => setConfirmation(event.target.value)} /></label>
          <div><button className="button button-secondary" type="button" onClick={() => setDeleteOpen(false)}>Cancel</button><button className="button danger-button" type="button" disabled={confirmation !== "invoice-approvals"}>Delete application</button></div>
        </section>
      </div>}
    </section>
  );
}

