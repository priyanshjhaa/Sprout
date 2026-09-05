import { AccessView } from "@/components/access/access-view";

export default async function AccessPage({ params }: { params: Promise<{ workspaceSlug: string; appId: string }> }) {
  const { workspaceSlug, appId } = await params;
  return <AccessView workspaceSlug={workspaceSlug} appId={appId} />;
}

