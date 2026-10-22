import { Band } from "@/components/marketing/band";
import { Chapters } from "@/components/marketing/chapters";
import { Hero } from "@/components/marketing/hero";
import { RevealObserver } from "@/components/marketing/reveal-observer";
import { SiteNav } from "@/components/marketing/site-nav";
import { Statement } from "@/components/marketing/statement";
import { WaysIn } from "@/components/marketing/ways-in";

export function LandingPage() {
  return (
    <div className="lp">
      <SiteNav />
      <main>
        <Hero />
        <Statement />
        <Chapters />
        <WaysIn />
        <Band />
      </main>
      <RevealObserver />
    </div>
  );
}
