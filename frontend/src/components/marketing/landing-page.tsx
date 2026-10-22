import { Hero } from "@/components/marketing/hero";
import { RevealObserver } from "@/components/marketing/reveal-observer";
import { SiteNav } from "@/components/marketing/site-nav";

export function LandingPage() {
  return (
    <div className="lp">
      <SiteNav />
      <main>
        <Hero />
      </main>
      <RevealObserver />
    </div>
  );
}
