import { ScrollText } from "lucide-react";

// Runtime logs need a running application; until real deployments exist this stays an honest placeholder.
export function LogViewer() {
  return (
    <section className="app-section">
      <header className="section-heading"><div><h2>Logs</h2><p>What the running application is doing.</p></div></header>
      <div className="soft-empty">
        <ScrollText size={20} />
        <h2>Logs arrive with real deployments</h2>
        <p>Once this application runs, its output appears here to search and filter. Deployment simulations do not run code, so they produce no logs.</p>
      </div>
    </section>
  );
}
