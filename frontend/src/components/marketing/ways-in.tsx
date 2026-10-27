import { Bot, LayoutDashboard, Plug, Terminal } from "lucide-react";
import { ActionLink } from "@/components/marketing/action-link";

const ways = [
  {
    numeral: "I",
    title: "The Dashboard",
    line: "Drop a folder, watch it build",
    icon: LayoutDashboard,
    tone: "moss",
    available: true,
  },
  {
    numeral: "II",
    title: "The CLI",
    line: "sprout deploy from any folder",
    icon: Terminal,
    tone: "clay",
    available: true,
  },
  {
    numeral: "III",
    title: "The MCP Tool",
    line: "A folder or a GitHub commit",
    icon: Plug,
    tone: "slate",
    available: false,
  },
  {
    numeral: "IV",
    title: "Coding Agents",
    line: "Ship what your agent just wrote",
    icon: Bot,
    tone: "ochre",
    available: false,
  },
];

export function WaysIn() {
  return (
    <section className="lp-ways" id="ways-in" aria-labelledby="ways-title">
      <div className="lp-container">
        <header className="lp-ways-head" data-reveal>
          <h2 id="ways-title" className="lp-heading">
            Four ways in, one way to run
          </h2>
          <p className="lp-lede">
            Every client sends the same source archive to the same API, so an app deployed by an agent behaves exactly
            like one you uploaded yourself.
          </p>
          <ActionLink href="/start" arrow>
            Deploy from the dashboard
          </ActionLink>
        </header>

        <ul className="lp-books">
          {ways.map((way, index) => {
            const Icon = way.icon;
            return (
              <li key={way.title} data-reveal style={{ "--i": index } as React.CSSProperties}>
                <div className="lp-book" data-tone={way.tone} aria-hidden="true">
                  <div className="lp-book-cover">
                    <span className="lp-book-numeral">{way.numeral}</span>
                    <Icon className="lp-book-icon" size={34} strokeWidth={1.3} />
                    <span className="lp-book-title">{way.title}</span>
                    <span className="lp-book-by">Sprout field guide</span>
                  </div>
                  <div className="lp-book-pages" />
                </div>
                <p className="lp-mono">Way {way.numeral}</p>
                <h3>{way.title}</h3>
                <p className="lp-book-line">{way.line}</p>
                <span className="lp-book-status" data-available={way.available || undefined}>
                  {way.available ? "Available now" : "Coming soon"}
                </span>
              </li>
            );
          })}
        </ul>
      </div>
    </section>
  );
}
