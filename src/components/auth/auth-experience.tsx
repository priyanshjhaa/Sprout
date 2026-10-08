import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import { Brand } from "@/components/brand";

const copy = {
  "sign-in": {
    eyebrow: "Welcome back",
    title: "Your apps are where you left them.",
    description: "Deploy, share, and look after the small apps your team relies on.",
  },
  "sign-up": {
    eyebrow: "Start deploying",
    title: "Give your app somewhere to live.",
    description: "Bring code you already have. Sprout builds it sealed off, checks its health, and gives your team a place to use it.",
  },
} as const;

export const authAppearance = {
  options: { elevation: "flush" },
  variables: {
    colorPrimary: "#687642",
    colorForeground: "#302b25",
    colorMutedForeground: "#70675d",
    colorBackground: "#fffaf2",
    colorInput: "#fffaf2",
    fontFamily: "inherit",
    borderRadius: "10px",
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
        <Link href="/"><ArrowLeft size={15} /> Back to Sprout</Link>
      </header>

      <div className="auth-layout">
        <section className="auth-story">
          <p className="eyebrow">{content.eyebrow}</p>
          <h1>{content.title}</h1>
          <p className="auth-story-description">{content.description}</p>
        </section>

        <section className="auth-form-panel" aria-label={mode === "sign-in" ? "Sign in" : "Sign up"}>
          {children}
        </section>
      </div>
    </main>
  );
}
