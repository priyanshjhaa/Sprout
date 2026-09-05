"use client";

import { ArrowRight, Check, Database, FileCode2, LockKeyhole, ScrollText, Users } from "lucide-react";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { Brand } from "@/components/brand";
import { GrowthSystem } from "@/components/marketing/growth-system";

const chapters = [
  {
    eyebrow: "Small software, finally at home",
    title: "The useful app in your head should be live by lunch.",
    body: "Sprout gives purpose-built software a safe place to run, without turning every idea into an infrastructure project.",
  },
  {
    eyebrow: "Bring the code",
    title: "Start with the application you already made.",
    body: "Connect a repository or hand Sprout the result of an agent session. We begin where code generation ends.",
  },
  {
    eyebrow: "A calm path to production",
    title: "Build, isolate, check, and publish.",
    body: "One clear deployment flow replaces the usual maze of container, proxy, certificate, and runtime configuration.",
  },
  {
    eyebrow: "Live",
    title: "A real URL, ready for real work.",
    body: "Sprout turns a local application into a healthy HTTPS service your team can open immediately.",
  },
  {
    eyebrow: "Everything it needs",
    title: "Infrastructure grows around the app.",
    body: "Attach data, storage, secrets, and logs without leaving the application or learning a new cloud vocabulary.",
  },
  {
    eyebrow: "Share the useful thing",
    title: "Invite coworkers like you would share a document.",
    body: "Workspace identity and simple roles make internal applications useful without rebuilding authentication every time.",
  },
  {
    eyebrow: "A cloud for the small things",
    title: "Give every workflow exactly the software it needs.",
    body: "One quiet workspace for experiments, internal tools, and agent-built applications that matter to a handful of people.",
  },
];

export function LandingExperience() {
  const [activeScene, setActiveScene] = useState(0);
  const storyRef = useRef<HTMLElement>(null);

  useEffect(() => {
    const elements = Array.from(document.querySelectorAll<HTMLElement>("[data-chapter]"));
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0];
        if (visible) setActiveScene(Number((visible.target as HTMLElement).dataset.chapter));
      },
      { rootMargin: "-12% 0px -20% 0px", threshold: [0, 0.1, 0.25] },
    );

    elements.forEach((element) => observer.observe(element));
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    const story = storyRef.current;
    if (!story) return;

    let frame = 0;
    let currentProgress = 0;
    let targetProgress = 0;
    const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
    const setProgress = (progress: number) => {
      const segment = (start: number, end: number) =>
        Math.min(1, Math.max(0, (progress - start) / (end - start))).toFixed(4);
      story.style.setProperty("--growth-progress", progress.toFixed(4));
      story.style.setProperty("--root-growth", segment(0.02, 0.28));
      story.style.setProperty("--stem-growth", segment(0.16, 0.48));
      story.style.setProperty("--leaf-growth", segment(0.38, 0.67));
      story.style.setProperty("--network-growth", segment(0.62, 0.9));
    };
    const readTarget = () => {
      const bounds = story.getBoundingClientRect();
      const distance = Math.max(1, story.offsetHeight - window.innerHeight);
      targetProgress = Math.min(1, Math.max(0, -bounds.top / distance));
    };
    const animate = () => {
      const difference = targetProgress - currentProgress;
      currentProgress += difference * 0.16;
      if (Math.abs(difference) < 0.0005) currentProgress = targetProgress;
      setProgress(currentProgress);
      frame = currentProgress === targetProgress ? 0 : window.requestAnimationFrame(animate);
    };
    const requestUpdate = () => {
      readTarget();
      if (reduceMotion.matches) {
        currentProgress = targetProgress;
        setProgress(currentProgress);
      } else if (!frame) {
        frame = window.requestAnimationFrame(animate);
      }
    };

    readTarget();
    currentProgress = targetProgress;
    setProgress(currentProgress);
    window.addEventListener("scroll", requestUpdate, { passive: true });
    window.addEventListener("resize", requestUpdate);
    return () => {
      window.removeEventListener("scroll", requestUpdate);
      window.removeEventListener("resize", requestUpdate);
      if (frame) window.cancelAnimationFrame(frame);
    };
  }, []);

  return (
    <main className="landing-shell">
      <header className="landing-nav">
        <Brand />
        <div className="landing-nav-actions">
          <a href="#story">How it works</a>
          <Link className="button button-small button-quiet" href="/sign-in">
            Sign in
          </Link>
          <Link className="button button-small button-primary" href="/workspace/acme/agent">
            Enter demo
          </Link>
        </div>
      </header>

      <section className="scroll-story" id="story" ref={storyRef} aria-label="How Sprout works">
        <div className="story-stage" data-scene={activeScene}>
          <div className="story-landscape" />
          <div className="growth-stage" aria-hidden="true">
            <GrowthSystem />
          </div>
          <div className="ambient-orb ambient-orb-one" />
          <div className="ambient-orb ambient-orb-two" />
          <span className="sr-only" aria-live="polite">{chapters[activeScene].title}</span>
          <div className="story-copy-stack">
            {chapters.map((chapter, index) => (
              <div
                className={`story-copy ${index === activeScene ? "is-active" : ""}`}
                key={chapter.eyebrow}
                aria-hidden={index !== activeScene}
              >
                <p className="eyebrow">{chapter.eyebrow}</p>
                <h1>{chapter.title}</h1>
                <p className="story-body">{chapter.body}</p>
                {index === activeScene && index === 0 && (
                  <div className="story-actions">
                    <Link className="button button-primary" href="/workspace/acme/agent">
                      Explore Sprout <ArrowRight size={16} />
                    </Link>
                    <span className="scroll-note">Scroll to grow the app</span>
                  </div>
                )}
                {index === activeScene && index === 6 && (
                  <div className="story-actions final-actions">
                    <Link className="button button-primary" href="/workspace/acme/agent">
                      Build something small <ArrowRight size={16} />
                    </Link>
                    <Link className="button button-secondary" href="/workspace/acme/apps">
                      View the workspace
                    </Link>
                  </div>
                )}
              </div>
            ))}
          </div>

          <div className="product-world" aria-hidden="true">
            <div className="source-card">
              <div className="source-icon"><FileCode2 size={18} /></div>
              <div><strong>invoice-approval</strong><span>Next.js · ready</span></div>
              <span className="source-ready"><Check size={13} /></span>
            </div>

            <div className="pipeline">
              <span>Build</span><i /><span>Isolate</span><i /><span>Check</span>
            </div>

            <div className="app-window">
              <div className="browser-bar"><i /><i /><i /><span>invoice-acme.sprout.run</span></div>
              <div className="invoice-ui">
                <div className="invoice-top"><span>Invoices</span><b>3 awaiting review</b></div>
                <div className="invoice-row"><i /><span>Northstar Labs<small>$2,480 · Sep 02</small></span><b>Review</b></div>
                <div className="invoice-row"><i /><span>Paper & Co.<small>$680 · Sep 01</small></span><b>Approved</b></div>
                <div className="invoice-row"><i /><span>Shape Systems<small>$1,205 · Aug 29</small></span><b>Review</b></div>
              </div>
            </div>

            <div className="resource resource-db"><Database size={16} /><span>Postgres</span></div>
            <div className="resource resource-secret"><LockKeyhole size={16} /><span>Secrets</span></div>
            <div className="resource resource-logs"><ScrollText size={16} /><span>Live logs</span></div>

            <div className="team-cluster">
              <div className="team-label"><Users size={15} /> Shared with your team</div>
              <div className="person person-one">PJ</div>
              <div className="person person-two">AC</div>
              <div className="person person-three">BH</div>
            </div>

            <div className="app-constellation">
              <span className="mini-app mini-one">Hiring</span>
              <span className="mini-app mini-two">Research</span>
              <span className="mini-app mini-three">Inventory</span>
            </div>
          </div>

          <div className="scene-progress" aria-label={`Story scene ${activeScene + 1} of ${chapters.length}`}>
            {chapters.map((chapter, index) => (
              <span key={chapter.eyebrow} className={index === activeScene ? "active" : ""} />
            ))}
          </div>
        </div>

        <div className="story-chapters">
          {chapters.map((chapter, index) => (
            <section key={chapter.eyebrow} data-chapter={index} aria-label={chapter.eyebrow}>
              <div className="mobile-story-copy">
                <p className="eyebrow">{chapter.eyebrow}</p>
                <h2>{chapter.title}</h2>
                <p>{chapter.body}</p>
              </div>
            </section>
          ))}
        </div>
      </section>
    </main>
  );
}
