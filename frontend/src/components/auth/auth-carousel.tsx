"use client";

import { Check, Globe, LockKeyhole, Moon, Play, Stethoscope, Zap } from "lucide-react";
import { useEffect, useState } from "react";

const INTERVAL = 6000;

function DeployVisual() {
  const stages = [
    { icon: LockKeyhole, label: "Build, sealed off", detail: "no network · 2 min limit" },
    { icon: Play, label: "Start", detail: "non-root · read-only" },
    { icon: Stethoscope, label: "Health check", detail: "GET /health → 200" },
  ];
  return (
    <ol className="auth-visual auth-visual-stages">
      {stages.map((stage, index) => {
        const Icon = stage.icon;
        return (
          <li key={stage.label} style={{ "--i": index } as React.CSSProperties}>
            <span className="auth-visual-icon">
              <Icon size={14} />
            </span>
            <span>
              <b>{stage.label}</b>
              <small>{stage.detail}</small>
            </span>
            <Check className="auth-visual-check" size={15} />
          </li>
        );
      })}
    </ol>
  );
}

function LiveVisual() {
  return (
    <div className="auth-visual auth-visual-live">
      <div className="auth-visual-browser">
        <i />
        <i />
        <i />
        <span>
          <Globe size={11} /> invoice-approval.acme.sprout.run
        </span>
      </div>
      <div className="auth-visual-live-body">
        <span className="auth-visual-badge">Live</span>
        <b>invoice-approval</b>
        <small>Answered /health in 41 ms</small>
      </div>
    </div>
  );
}

function SleepVisual() {
  return (
    <div className="auth-visual auth-visual-sleep">
      {Array.from({ length: 18 }, (_, index) => (
        <i key={index} style={{ animationDelay: `${((index * 7) % 11) * 0.45}s` }} />
      ))}
      <p>
        <Moon size={13} /> Idle apps sleep <Zap size={13} /> visits wake them
      </p>
    </div>
  );
}

function TeamVisual() {
  const members = [
    { initials: "PJ", name: "Priya", role: "Owner" },
    { initials: "AC", name: "Alex", role: "Editor" },
    { initials: "BH", name: "Bea", role: "Viewer" },
  ];
  return (
    <ul className="auth-visual auth-visual-team">
      {members.map((member, index) => (
        <li key={member.initials} style={{ "--i": index } as React.CSSProperties}>
          <i>{member.initials}</i>
          <span>{member.name}</span>
          <em>{member.role}</em>
        </li>
      ))}
    </ul>
  );
}

const slides = [
  {
    title: "Hand Sprout the code",
    body: "Upload a folder and Sprout builds it in a sealed container, starts it locked down, and checks that it answers.",
    visual: DeployVisual,
  },
  {
    title: "Healthy before it's public",
    body: "Only an app that passes its health check gets a public URL you can send to anyone.",
    visual: LiveVisual,
  },
  {
    title: "Quiet when nobody is looking",
    body: "Apps nobody is visiting go to sleep and wake on the next request, so many small apps stay cheap.",
    visual: SleepVisual,
  },
  {
    title: "Your team stays in control",
    body: "Owners, editors, and viewers decide who can deploy, pause, or archive each app.",
    visual: TeamVisual,
  },
];

export function AuthCarousel() {
  const [active, setActive] = useState(0);
  const [paused, setPaused] = useState(false);

  useEffect(() => {
    if (paused || window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const timer = window.setTimeout(() => setActive((current) => (current + 1) % slides.length), INTERVAL);
    return () => window.clearTimeout(timer);
  }, [active, paused]);

  return (
    <div
      className="auth-carousel"
      role="region"
      aria-roledescription="carousel"
      aria-label="What Sprout does"
      data-paused={paused || undefined}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocus={() => setPaused(true)}
      onBlur={() => setPaused(false)}
    >
      <div className="auth-slides">
        {slides.map((slide, index) => {
          const Visual = slide.visual;
          return (
            <section
              key={slide.title}
              className="auth-slide"
              aria-roledescription="slide"
              aria-label={`${index + 1} of ${slides.length}`}
              aria-hidden={index !== active}
              data-active={index === active || undefined}
            >
              <h2>{slide.title}</h2>
              <p>{slide.body}</p>
              <Visual />
            </section>
          );
        })}
      </div>
      <div className="auth-dots">
        {slides.map((slide, index) => (
          <button
            key={slide.title}
            type="button"
            aria-label={`Show slide ${index + 1}: ${slide.title}`}
            aria-current={index === active || undefined}
            onClick={() => setActive(index)}
            style={{ "--interval": `${INTERVAL}ms` } as React.CSSProperties}
          />
        ))}
      </div>
    </div>
  );
}
