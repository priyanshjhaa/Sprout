import Link from "next/link";
import { Brand } from "@/components/brand";
import { ActionLink } from "@/components/marketing/action-link";

const sprouts = [6, 17, 29, 41, 55, 66, 78, 90];

export function SiteFooter() {
  return (
    <footer className="lp-footer">
      <div className="lp-container lp-footer-top">
        <div className="lp-footer-cta" data-reveal>
          <h2 className="lp-heading">Give your small apps somewhere to live</h2>
          <ActionLink href="/start" arrow>
            Deploy an app
          </ActionLink>
        </div>

        <nav className="lp-footer-links" aria-label="Footer">
          <div>
            <p className="lp-mono">How it works</p>
            <a href="#deploy">Deploy</a>
            <a href="#operate">Operate</a>
            <a href="#share">Share</a>
          </div>
          <div>
            <p className="lp-mono">Sprout</p>
            <a href="#ways-in">Ways in</a>
            <Link href="/sign-in">Sign in</Link>
            <Link href="/sign-up">Create an account</Link>
          </div>
        </nav>
      </div>

      <div className="lp-footer-ground" data-reveal>
        <div className="lp-footer-sprouts" aria-hidden="true">
          {sprouts.map((left, index) => (
            <svg
              key={left}
              viewBox="0 0 40 60"
              style={{ left: `${left}%`, "--i": index } as React.CSSProperties}
            >
              <path className="lp-mini-stem" d="M20 60 C20 48 19 40 20 28" />
              <path className="lp-mini-leaf" d="M20 36 C13 35 8 30 7 22 C14 22 19 28 20 36 Z" />
              <path className="lp-mini-leaf lp-mini-leaf--right" d="M20 31 C26 28 32 22 33 15 C26 16 21 22 20 31 Z" />
            </svg>
          ))}
        </div>
        <div className="lp-container lp-footer-bottom">
          <Brand />
          <p>Small apps, safely online. © {new Date().getFullYear()} Sprout</p>
        </div>
      </div>
    </footer>
  );
}
