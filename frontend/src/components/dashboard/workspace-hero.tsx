import { ArrowRight, Plus } from "lucide-react";
import Link from "next/link";
import { LandscapeScene } from "@/components/marketing/landscape-scene";

// The overview's welcome card: copy on the left, Sprout's landscape fading in from the right.
export function WorkspaceHero({
  base,
  workspaceName,
  appCount,
  liveCount,
  isLoading,
}: {
  base: string;
  workspaceName: string;
  appCount: number;
  liveCount: number;
  isLoading: boolean;
}) {
  const summary = isLoading
    ? "Checking on your apps…"
    : appCount === 0
      ? "Plant your first app. Hand Sprout the code and it comes back as a healthy public URL."
      : liveCount === 0
        ? "Nothing is live right now. Deploy an app or wake one that is resting."
        : `${liveCount} of ${appCount} ${appCount === 1 ? "app is" : "apps are"} live and answering.`;

  return (
    <section className="dash-hero" aria-labelledby="dash-hero-title">
      <div className="dash-hero-art" aria-hidden="true">
        <LandscapeScene />
        <svg className="lp-seedling dash-hero-seedling" viewBox="0 0 80 120">
          <path className="lp-seedling-stem" d="M40 120 C40 96 38 78 40 52" />
          <path className="lp-seedling-leaf lp-seedling-leaf--left" d="M40 66 C26 64 16 54 14 40 C28 40 38 50 40 66 Z" />
          <path className="lp-seedling-leaf lp-seedling-leaf--right" d="M40 56 C52 50 64 40 66 26 C52 28 42 38 40 56 Z" />
        </svg>
      </div>
      <div className="dash-hero-copy">
        <p className="dash-hero-eyebrow">Welcome back</p>
        <h1 id="dash-hero-title">{workspaceName}</h1>
        <p className="dash-hero-summary">{summary}</p>
        <div className="dash-hero-actions">
          <Link className="lp-btn lp-btn--dark" href={`${base}/deploy`}>
            <Plus size={16} aria-hidden="true" /> Deploy an app
          </Link>
          {appCount > 0 && (
            <Link className="lp-btn lp-btn--light" href={`${base}/apps`}>
              All apps <ArrowRight size={15} aria-hidden="true" />
            </Link>
          )}
        </div>
      </div>
    </section>
  );
}
