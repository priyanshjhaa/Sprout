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

    const observeWithin = (root: ParentNode) => {
      root.querySelectorAll("[data-reveal]:not([data-shown])").forEach((element) => reveal.observe(element));
      root.querySelectorAll("[data-animate]").forEach((element) => play.observe(element));
    };
    observeWithin(document);

    // Sections rendered after this effect (client components, remounts) are picked up too.
    const added = new MutationObserver((records) => {
      for (const record of records) {
        record.addedNodes.forEach((node) => {
          if (!(node instanceof Element)) return;
          if (node.matches("[data-reveal]:not([data-shown])")) reveal.observe(node);
          if (node.matches("[data-animate]")) play.observe(node);
          observeWithin(node);
        });
      }
    });
    added.observe(document.body, { childList: true, subtree: true });

    return () => {
      added.disconnect();
      reveal.disconnect();
      play.disconnect();
    };
  }, []);

  return null;
}
