import { redirect } from "next/navigation";

// Not part of the current scope (see deployment-strategy.md); keep old links useful.
export default async function RemovedApplicationPage({ params }: { params: Promise<{ workspaceSlug: string; appId: string }> }) {
  const { workspaceSlug, appId } = await params;
  redirect(`/workspace/${workspaceSlug}/apps/${appId}`);
}
