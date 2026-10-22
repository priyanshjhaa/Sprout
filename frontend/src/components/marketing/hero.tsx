import { ActionLink } from "@/components/marketing/action-link";
import { LandscapeScene } from "@/components/marketing/landscape-scene";
import { PixelEdge } from "@/components/marketing/pixel-edge";

const headline = "Sprout gives your small apps a safe place to live";

export function Hero() {
  return (
    <section className="lp-hero" id="hero" aria-labelledby="hero-title">
      <LandscapeScene />

      <div className="lp-container lp-hero-copy">
        <h1 id="hero-title" className="lp-hero-title">
          {headline.split(" ").map((word, index) => (
            <span key={index} style={{ "--w": index } as React.CSSProperties}>
              {word}{" "}
            </span>
          ))}
        </h1>
        <p className="lp-hero-lede">
          Hand Sprout the code. It builds it sealed off, checks that it answers, and puts it at a public URL your
          team can run, pause, and retire.
        </p>
        <div className="lp-hero-actions">
          <ActionLink href="/start" arrow>
            Deploy an app
          </ActionLink>
          <ActionLink href="#how" variant="ghost">
            See how it works
          </ActionLink>
        </div>
      </div>

      <div className="lp-hero-seedling" aria-hidden="true">
        <div className="lp-status-chip lp-glass">
          <span className="lp-status-dot" />
          <span className="lp-status-app">invoice-approval</span>
          <span className="lp-status-cycle">
            <span>Building</span>
            <span>Checking health</span>
            <span>Live</span>
          </span>
        </div>
        <svg className="lp-seedling" viewBox="0 0 80 120">
          <path className="lp-seedling-stem" d="M40 120 C40 96 38 78 40 52" />
          <path className="lp-seedling-leaf lp-seedling-leaf--left" d="M40 66 C26 64 16 54 14 40 C28 40 38 50 40 66 Z" />
          <path className="lp-seedling-leaf lp-seedling-leaf--right" d="M40 56 C52 50 64 40 66 26 C52 28 42 38 40 56 Z" />
        </svg>
      </div>

      <PixelEdge />
    </section>
  );
}
