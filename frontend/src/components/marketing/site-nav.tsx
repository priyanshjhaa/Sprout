"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Brand } from "@/components/brand";
import { ActionLink } from "@/components/marketing/action-link";

const sections = [
  { href: "#deploy", label: "Deploy" },
  { href: "#operate", label: "Operate" },
  { href: "#share", label: "Share" },
];

// Glass over the hero, then a solid bar once the hero has scrolled away.
export function SiteNav() {
  const [solid, setSolid] = useState(false);

  useEffect(() => {
    const hero = document.getElementById("hero");
    if (!hero) return;
    const observer = new IntersectionObserver(([entry]) => setSolid(!entry.isIntersecting), {
      rootMargin: "-72px 0px 0px 0px",
    });
    observer.observe(hero);
    return () => observer.disconnect();
  }, []);

  return (
    <header className="lp-nav" data-solid={solid || undefined}>
      <div className="lp-nav-inner">
        <Brand />
        <nav className="lp-nav-links" aria-label="Sections">
          <div className="lp-nav-group">
            <span className="lp-nav-caption">How it works</span>
            {sections.map((section) => (
              <a key={section.href} href={section.href}>
                {section.label}
              </a>
            ))}
          </div>
          <div className="lp-nav-group">
            <a href="#ways-in">Ways in</a>
          </div>
          <div className="lp-nav-group">
            <Link href="/sign-in">Sign in</Link>
          </div>
        </nav>
        <ActionLink href="/start" variant={solid ? "dark" : "light"} small>
          Deploy an app
        </ActionLink>
      </div>
    </header>
  );
}
