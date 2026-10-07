"use client";

import { ArrowRight, Check, Database, FileCode2, LockKeyhole, ScrollText, Users } from "lucide-react";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { Brand } from "@/components/brand";
import { GrowthSystem } from "@/components/marketing/growth-system";

const chapters = [
  {
    eyebrow: "Small software, finally at home",
    title: "Your app works. Now give it somewhere to live.",
    body: "Sprout takes application code you already have, from your editor, a teammate, or a coding agent, and turns it into a running app your team can open.",
  },
  {
    eyebrow: "Plant the code",
    title: "Hand Sprout the code. Nothing else.",
    body: "Upload it, push it from the CLI, or let a coding agent deploy it. No Dockerfile, no proxy, no cloud console.",
  },
  {
    eyebrow: "Sealed underground",
    title: "Every build runs sealed off.",
    body: "Untrusted code builds in a disposable container with no network, fixed CPU and memory, and a hard time limit, kept away from everything else you run.",
  },
  {
    eyebrow: "Breaking the surface",
    title: "Healthy first. Then it gets a URL.",
    body: "Sprout starts the app, checks that it answers on /health, and only then routes your team to its address.",
  },
  {
    eyebrow: "Everything it needs",
    title: "Data, secrets, and logs grow around it.",
    body: "Attach a database, keep secrets out of the code, and read its logs without leaving the app or learning a new cloud vocabulary.",
  },
  {
    eyebrow: "Share the useful thing",
    title: "Share it like a document.",
    body: "Invite coworkers as viewers or editors, or restrict an app to the few people who need it. Their access ends when they leave the workspace.",
  },
  {
    eyebrow: "A garden, not a server farm",
    title: "Many small apps, each with a clear life.",
    body: "Live apps serve. Resting apps pause and release what they use. Finished apps are archived, and deleting one really deletes it.",
  },
];

export function LandingExperience() {
  const [activeScene, setActiveScene] = useState(0);
  const storyRef = useRef<HTMLElement>(null);

  useEffect(() => {
    const story = storyRef.current;
    if (!story) return;

    let frame = 0;
    let currentProgress = 0;
    let targetProgress = 0;
    let currentScene = 0;
    let lastFrameTime = performance.now();
    const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
    const setProgress = (progress: number) => {
      const segment = (start: number, end: number) => {
        const value = Math.min(1, Math.max(0, (progress - start) / (end - start)));
        return value * value * (3 - 2 * value);
      };
      const set = (name: string, value: number) => story.style.setProperty(name, value.toFixed(4));
      set("--growth-progress", progress);
      // The camera sinks below the soil while the build runs, then rises with the healthy app.
      set("--camera-depth", segment(0.12, 0.3) * (1 - segment(0.4, 0.52)));
      set("--seed-open", segment(0.01, 0.14));
      set("--root-growth", segment(0.1, 0.34));
      set("--root-fine-growth", segment(0.2, 0.38));
      set("--stem-growth", segment(0.36, 0.52));
      set("--lower-branch-growth", segment(0.44, 0.54));
      set("--upper-branch-growth", segment(0.48, 0.58));
      set("--leaf-growth", segment(0.48, 0.7));
      set("--lower-leaf-growth", segment(0.5, 0.62));
      set("--upper-leaf-growth", segment(0.56, 0.7));
      set("--network-growth", segment(0.72, 0.92));
    };
    const readTarget = () => {
      const bounds = story.getBoundingClientRect();
      const distance = Math.max(1, story.offsetHeight - window.innerHeight);
      targetProgress = Math.min(1, Math.max(0, -bounds.top / distance));
      const nextScene = Math.min(chapters.length - 1, Math.round(targetProgress * (chapters.length - 1)));
      if (nextScene !== currentScene) {
        currentScene = nextScene;
        setActiveScene(nextScene);
      }
    };
    const animate = (time: number) => {
      const elapsed = Math.min(64, time - lastFrameTime);
      lastFrameTime = time;
      const difference = targetProgress - currentProgress;
      const easing = 1 - Math.exp(-elapsed / 82);
      currentProgress += difference * easing;
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
        lastFrameTime = performance.now();
        frame = window.requestAnimationFrame(animate);
      }
    };

    readTarget();
    currentProgress = targetProgress;
    setProgress(currentProgress);
    window.addEventListener("scroll", requestUpdate, { passive: true });
    window.addEventListener("resize", requestUpdate);
    reduceMotion.addEventListener("change", requestUpdate);
    return () => {
      window.removeEventListener("scroll", requestUpdate);
      window.removeEventListener("resize", requestUpdate);
      reduceMotion.removeEventListener("change", requestUpdate);
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
          <Link className="button button-small button-primary" href="/start">
            Open workspace
          </Link>
        </div>
      </header>

      <section className="scroll-story" id="story" ref={storyRef} aria-label="How Sprout works">
        <div className="story-stage" data-scene={activeScene}>
          <div className="story-world" aria-hidden="true">
            <div className="world-layer world-sky" />
            <div className="world-light" />
            <div className="world-veil" />
            <div className="world-layer world-ground">
              <div className="ground-soil" />
              <div className="growth-stage">
                <GrowthSystem />
              </div>
              <div className="ground-grass ground-grass-left" />
              <div className="ground-grass ground-grass-right" />
              <div className="ground-depth">
                <div className="build-chamber">
                  <p className="chamber-label"><LockKeyhole size={13} /> Sealed build · invoice-approval</p>
                  <div className="pipeline">
                    <span>Build</span><i /><span>Isolate</span><i /><span>Check</span>
                  </div>
                  <ul className="chamber-limits">
                    <li>No network</li><li>1 CPU</li><li>1 GiB memory</li><li>2 min limit</li>
                  </ul>
                </div>
              </div>
            </div>
          </div>
          <div className="growth-atmosphere" aria-hidden="true">
            {Array.from({ length: 8 }, (_, index) => <i key={index} />)}
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
                    <Link className="button button-primary" href="/start">
                      Deploy an app <ArrowRight size={16} />
                    </Link>
                    <span className="scroll-note">Scroll to plant it</span>
                  </div>
                )}
                {index === activeScene && index === 6 && (
                  <div className="story-actions final-actions">
                    <Link className="button button-primary" href="/start">
                      Deploy an app <ArrowRight size={16} />
                    </Link>
                    <Link className="button button-secondary" href="/start">
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
              <div><strong>invoice-approval</strong><span>Node.js · ready to deploy</span></div>
              <span className="source-ready"><Check size={13} /></span>
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
              <span className="mini-app mini-one" data-state="live">Hiring<small>Live</small></span>
              <span className="mini-app mini-two" data-state="paused">Research<small>Paused</small></span>
              <span className="mini-app mini-three" data-state="archived">Inventory<small>Archived</small></span>
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
