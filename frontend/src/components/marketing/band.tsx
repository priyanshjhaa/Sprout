import { FileLock2, Gauge, HardDrive, ScrollText, ShieldOff, UserX } from "lucide-react";
import { PixelEdge } from "@/components/marketing/pixel-edge";

const garden = Array.from({ length: 24 }, (_, index) => ({
  // A deterministic scatter of apps that fall asleep and wake at different times.
  delay: `${((index * 7) % 11) * 0.6}s`,
  duration: `${6 + ((index * 5) % 4)}s`,
}));

const details = [
  { icon: UserX, label: "Non-root user" },
  { icon: FileLock2, label: "Read-only root filesystem" },
  { icon: ShieldOff, label: "No Linux capabilities" },
  { icon: Gauge, label: "Fixed CPU and memory" },
  { icon: ScrollText, label: "Logs for every build" },
  { icon: HardDrive, label: "Release mounted read-only" },
];

export function Band() {
  return (
    <section className="lp-band" aria-labelledby="band-title">
      <PixelEdge className="lp-pixel-edge--top" />
      <div className="lp-band-glow" aria-hidden="true" />
      <div className="lp-container">
        <header className="lp-band-head" data-reveal>
          <h2 id="band-title" className="lp-heading">
            Everything a small app needs, nothing it doesn&apos;t
          </h2>
          <p>
            Untrusted code gets a sealed build, a locked-down container, and a health check before anyone can reach
            it. Then it simply runs.
          </p>
        </header>

        <div className="lp-band-cards">
          <article className="lp-band-card" data-reveal>
            <div className="lp-band-art lp-pulse" aria-hidden="true">
              <svg viewBox="0 0 300 100" preserveAspectRatio="none">
                <path d="M0 60 H90 L105 60 L115 25 L128 85 L140 45 L150 60 H300" />
              </svg>
              <span>GET /health → 200</span>
            </div>
            <h3>Nothing goes public until it&apos;s healthy</h3>
            <p>Sprout routes visitors to an app only after it answers on /health.</p>
          </article>

          <article className="lp-band-card" data-reveal style={{ "--i": 1 } as React.CSSProperties}>
            <div className="lp-band-art lp-garden" aria-hidden="true">
              {garden.map((tile, index) => (
                <i key={index} style={{ animationDelay: tile.delay, animationDuration: tile.duration }} />
              ))}
            </div>
            <h3>Many apps, side by side</h3>
            <p>Each app runs on its own and sleeps when idle, so a whole garden of them stays cheap.</p>
          </article>

          <article className="lp-band-card" data-reveal style={{ "--i": 2 } as React.CSSProperties}>
            <div className="lp-band-art lp-sealed" aria-hidden="true">
              <div className="lp-sealed-box">
                <i />
                <i />
                <i />
                <i />
              </div>
              <span className="lp-sealed-wall">no network</span>
            </div>
            <h3>Every build sealed off</h3>
            <p>Builds run in a disposable container with no network, no host mounts, and a hard time limit.</p>
          </article>
        </div>

        <ul className="lp-band-details">
          {details.map((detail, index) => {
            const Icon = detail.icon;
            return (
              <li key={detail.label} data-reveal style={{ "--i": index } as React.CSSProperties}>
                <Icon size={16} aria-hidden="true" />
                {detail.label}
              </li>
            );
          })}
        </ul>
      </div>
      <PixelEdge />
    </section>
  );
}
