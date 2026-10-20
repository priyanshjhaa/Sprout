import type { ReactNode } from "react";
import { auth } from "@clerk/nextjs/server";
import { redirect } from "next/navigation";
import { DashboardShell } from "@/components/dashboard/dashboard-shell";
import { SessionUser } from "@/lib/query/session";

export default async function WorkspaceLayout({ children }: { children: ReactNode }) {
  const { userId } = await auth();
  if (!userId) redirect("/sign-in");
  return <SessionUser id={userId}><DashboardShell>{children}</DashboardShell></SessionUser>;
}
