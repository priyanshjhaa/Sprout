import { Activity } from "lucide-react";

export default function ActivityPage() {
  return (
    <main className="dashboard-page compact-page">
      <header className="page-heading"><p className="eyebrow">Workspace</p><h1>Activity</h1><p>A quiet record of the work happening across your applications.</p></header>
      <section className="soft-empty"><Activity size={20} /><h2>Nothing new to review</h2><p>Deployments, invitations, and configuration changes will appear here.</p></section>
    </main>
  );
}

