import type { ReactNode } from "react";
import { ApplicationShell } from "@/components/apps/application-shell";

export default function ApplicationLayout({ children }: { children: ReactNode }) {
  return <ApplicationShell>{children}</ApplicationShell>;
}

