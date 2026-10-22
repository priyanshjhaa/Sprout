"use client";

import { useEffect, useRef, useState } from "react";

// Steps through a mock's story while it is on screen, then loops.
// With reduced motion it shows the finished state and stays there.
export function useLoopStep<T extends HTMLElement>(steps: number, interval: number, hold = 2) {
  const ref = useRef<T>(null);
  const [step, setStep] = useState(0);

  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");

    let timer: number | undefined;
    // The last step lingers for `hold` intervals before the story restarts.
    const total = steps + hold;
    let tick = 0;
    const start = () => {
      if (reduceMotion.matches) {
        setStep(steps - 1);
        return;
      }
      if (timer !== undefined) return;
      timer = window.setInterval(() => {
        tick = (tick + 1) % total;
        setStep(Math.min(tick, steps - 1));
      }, interval);
    };
    const stop = () => {
      window.clearInterval(timer);
      timer = undefined;
    };

    const observer = new IntersectionObserver(([entry]) => (entry.isIntersecting ? start() : stop()), {
      threshold: 0.25,
    });
    observer.observe(element);
    return () => {
      observer.disconnect();
      stop();
    };
  }, [steps, interval, hold]);

  return [ref, step] as const;
}
