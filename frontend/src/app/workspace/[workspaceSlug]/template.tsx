import type { ReactNode } from "react";

// Templates remount on navigation, so each page rises softly into place while
// the bar above stays put.
export default function WorkspaceTemplate({ children }: { children: ReactNode }) {
  return <div className="sp-page-in">{children}</div>;
}
