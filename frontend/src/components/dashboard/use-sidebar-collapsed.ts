"use client";

import { useCallback, useSyncExternalStore } from "react";

const KEY = "sprout-sidebar-collapsed";
const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
  listeners.add(listener);
  window.addEventListener("storage", listener);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", listener);
  };
}

function read() {
  try {
    return window.localStorage.getItem(KEY) === "1";
  } catch {
    return false;
  }
}

// Whether the sidebar is folded down to icons, remembered in this browser.
export function useSidebarCollapsed() {
  const collapsed = useSyncExternalStore(subscribe, read, () => false);
  const toggle = useCallback(() => {
    try {
      window.localStorage.setItem(KEY, read() ? "0" : "1");
    } catch {
      // Storage can be unavailable; the toggle then lasts until the page reloads.
    }
    listeners.forEach((listener) => listener());
  }, []);
  return [collapsed, toggle] as const;
}
