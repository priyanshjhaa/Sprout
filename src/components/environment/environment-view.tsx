import { KeyRound } from "lucide-react";

// Variables need encrypted secret storage, which does not exist yet, so nothing can be saved here today.
export function EnvironmentView() {
  return (
    <section className="app-section">
      <header className="section-heading"><div><h2>Environment</h2><p>Configuration and secrets the application reads at runtime.</p></div></header>
      <div className="soft-empty">
        <KeyRound size={20} />
        <h2>Variables arrive with secret storage</h2>
        <p>You will set configuration and secrets here, kept out of the code and encrypted at rest. Nothing can be stored until that is in place.</p>
      </div>
    </section>
  );
}
