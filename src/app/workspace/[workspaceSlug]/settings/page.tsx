import { Settings } from "lucide-react";

export default function WorkspaceSettingsPage() {
  return (
    <main className="dashboard-page compact-page">
      <header className="page-heading"><p className="eyebrow">Workspace</p><h1>Settings</h1><p>Workspace identity and defaults, kept out of the way until you need them.</p></header>
      <section className="soft-empty"><Settings size={20} /><h2>No settings need attention</h2><p>Workspace configuration will grow only when the product requires it.</p></section>
    </main>
  );
}

