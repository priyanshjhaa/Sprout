import { ActionLink } from "@/components/marketing/action-link";
import { DeployMock } from "@/components/marketing/deploy-mock";

const facts = [
  "Builds run with no network",
  "Non-root, read-only containers",
  "Healthy on /health before it goes public",
  "Sleeps when idle, wakes on a visit",
  "Static sites and Node.js apps",
  "One deploy API behind every client",
  "Pause, resume, and archive any app",
];

const points = [
  {
    title: "Sealed builds",
    body: "Your code builds in a disposable container with no network, fixed CPU and memory, and a hard time limit.",
  },
  {
    title: "Healthy before public",
    body: "Sprout starts the app and waits for it to answer on /health. Only then does it get a public URL.",
  },
  {
    title: "Quiet when idle",
    body: "Apps nobody is using go to sleep and wake on the next visit, so many small apps cost almost nothing.",
  },
];

export function Statement() {
  return (
    <>
      <section className="lp-facts" aria-label="What every app gets">
        <p className="lp-facts-caption" data-reveal>
          Made for many small apps, not a few big ones
        </p>
        <div className="lp-marquee">
          {/* The list is repeated once so the loop has no seam; the copy is hidden from assistive tech. */}
          {[0, 1].map((copy) => (
            <ul key={copy} aria-hidden={copy === 1 || undefined}>
              {facts.map((fact) => (
                <li key={fact}>{fact}</li>
              ))}
            </ul>
          ))}
        </div>
      </section>

      <section className="lp-statement" id="how" aria-labelledby="statement-title">
        <div className="lp-container">
          <h2 id="statement-title" className="lp-heading lp-statement-title" data-reveal>
            Sprout takes code you already have
            <span> and turns it into an app anyone can open</span>
          </h2>

          <div className="lp-statement-mock" data-reveal style={{ "--i": 1 } as React.CSSProperties}>
            <DeployMock />
          </div>

          <ul className="lp-points">
            {points.map((point, index) => (
              <li key={point.title} data-reveal style={{ "--i": index } as React.CSSProperties}>
                <b>{point.title}</b> — {point.body}
              </li>
            ))}
          </ul>

          <div className="lp-statement-cta" data-reveal>
            <p>
              <b>Bring the code.</b> Sprout handles the build, the container, and the URL.
            </p>
            <ActionLink href="/start" arrow>
              Deploy an app
            </ActionLink>
          </div>
        </div>
      </section>
    </>
  );
}
