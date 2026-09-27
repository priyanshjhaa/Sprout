import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import { Brand } from "@/components/brand";

const copy = {
  "sign-in": {
    eyebrow: "Welcome back",
    title: "Your small software has a place to grow.",
    description: "Return to a quiet workspace for the useful applications you build, run, and share.",
  },
  "sign-up": {
    eyebrow: "A place to begin",
    title: "Make room for the next useful thing.",
    description: "Give purpose-built software a home from the first idea to the moment you share it.",
  },
} as const;

export const authAppearance = {
  options: { elevation: "flush" },
  variables: {
    colorPrimary: "#687642",
    colorForeground: "#302b25",
    colorBackground: "#fffaf2",
  },
  elements: {
    rootBox: { width: "100%" },
    cardBox: { width: "100%" },
    card: { width: "100%" },
  },
} as const;

export function AuthExperience({ mode, children }: { mode: keyof typeof copy; children: ReactNode }) {
  const content = copy[mode];

  return (
    <main className="auth-landscape-page">
      <div className="auth-landscape" aria-hidden="true" />
      <header className="auth-header">
        <Brand />
        <Link href="/"><ArrowLeft size={15} /> Back to the story</Link>
      </header>

      <div className="auth-layout">
        <section className="auth-story">
          <p className="eyebrow">{content.eyebrow}</p>
          <h1>{content.title}</h1>
          <p className="auth-story-description">{content.description}</p>
          <div className="auth-story-detail">
            <span className="seed-icon" aria-hidden="true"><span /><span /></span>
            <span>Small software, finally at home.</span>
          </div>
        </section>

        <section className="auth-form-panel" aria-label={mode === "sign-in" ? "Sign in" : "Sign up"}>
          <p className="auth-form-kicker">SPROUT / YOUR WORKSPACE</p>
          <div className="auth-clerk-form">{children}</div>
        </section>
      </div>

      <footer className="auth-footer">A calm place for every useful little app.</footer>
    </main>
  );
}
