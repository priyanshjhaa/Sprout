"use client";

import { useEffect } from "react";

// One observer for the whole landing page, so sections can stay server components.
// [data-reveal] elements fade in once; [data-animate] mocks play only while visible.
export function RevealObserver() {
  useEffect(() => {
    const reveal = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          entry.target.setAttribute("data-shown", "");
          reveal.unobserve(entry.target);
        }
      },
      { rootMargin: "0px 0px -10% 0px", threshold: 0.12 },
    );

    const play = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) entry.target.toggleAttribute("data-playing", entry.isIntersecting);
      },
      { threshold: 0.2 },
    );

    document.querySelectorAll("[data-reveal]").forEach((element) => reveal.observe(element));
    document.querySelectorAll("[data-animate]").forEach((element) => play.observe(element));

    return () => {
      reveal.disconnect();
      play.disconnect();
    };
  }, []);

  return null;
}
