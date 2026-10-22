import { ArrowLeft, LogIn, UserPlus } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import { AuthCarousel } from "@/components/auth/auth-carousel";
import { LandscapeScene } from "@/components/marketing/landscape-scene";

type Mode = "sign-in" | "sign-up" | "invitation" | "start" | "unavailable";

const titles: Record<Mode, string> = {
  "sign-in": "Sign in to Sprout",
  "sign-up": "Create your Sprout account",
  invitation: "Accept your invitation",
  start: "Opening your workspace",
  unavailable: "Workspace unavailable",
};

const taglines: Record<Mode, string> = {
  "sign-in": "Your apps are where you left them.",
  "sign-up": "Let's get your app online.",
  invitation: "Your team saved you a seat.",
  start: "Getting your garden ready.",
  unavailable: "This workspace is out of reach.",
};

function ModeSwitch({ mode, invitation }: { mode: "sign-in" | "sign-up"; invitation: boolean }) {
  const suffix = invitation ? "?invitation=1" : "";
  return (
    <nav className="auth-switch" data-mode={mode} aria-label="Authentication mode">
      <span className="auth-switch-thumb" aria-hidden="true" />
      <Link href={`/sign-in${suffix}`} aria-current={mode === "sign-in" ? "page" : undefined}>
        <LogIn size={13} aria-hidden="true" /> Sign In
      </Link>
      <Link href={`/sign-up${suffix}`} aria-current={mode === "sign-up" ? "page" : undefined}>
        <UserPlus size={13} aria-hidden="true" /> Sign Up
      </Link>
    </nav>
  );
}

// The shared frame for sign-in, sign-up, and invitation acceptance: a landscape
// showcase that tells Sprout's story on the left, the form on the right.
export function AuthShell({
  mode,
  invitation = false,
  children,
}: {
  mode: Mode;
  invitation?: boolean;
  children: ReactNode;
}) {
  const tagline = taglines[mode];
  return (
    <div className="lp auth">
      <aside className="auth-showcase">
        <LandscapeScene />
        <AuthCarousel />
      </aside>

      <div className="auth-panel">
        <header className="auth-head">
          <Link className="auth-wordmark" href="/" aria-label="Sprout home">
            <span className="brand-mark" aria-hidden="true">
              <span />
              <span />
            </span>
            Sprout
          </Link>
          <p className="auth-tagline">
            <span>{tagline}</span>
          </p>
        </header>

        <main className="auth-body">
          <h1 className="sr-only">{titles[mode]}</h1>
          {(mode === "sign-in" || mode === "sign-up") && <ModeSwitch mode={mode} invitation={invitation} />}
          <section className="auth-form" aria-label={titles[mode]}>
            {children}
          </section>
          <Link className="auth-back" href="/">
            <ArrowLeft size={14} aria-hidden="true" /> Back to Sprout
          </Link>
        </main>

        <footer className="auth-signoff">
          <span>Sprout</span>
          <span>Small apps, looked after</span>
        </footer>
      </div>
    </div>
  );
}
