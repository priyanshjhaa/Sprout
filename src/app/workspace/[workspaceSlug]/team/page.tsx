import { Users } from "lucide-react";

export default function TeamPage() {
  return (
    <main className="dashboard-page compact-page">
      <header className="page-heading"><p className="eyebrow">Workspace</p><h1>Team</h1><p>Manage the people who can build and use applications in this workspace.</p></header>
      <section className="soft-empty"><Users size={20} /><h2>Team management is planned</h2><p>Application-specific access remains available from each app.</p></section>
    </main>
  );
}

