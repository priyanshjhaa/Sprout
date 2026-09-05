import { EnvironmentView } from "@/components/environment/environment-view";

export default async function EnvironmentPage({ params }: { params: Promise<{ workspaceSlug: string; appId: string }> }) {
  const { workspaceSlug, appId } = await params;
  return <EnvironmentView workspaceSlug={workspaceSlug} appId={appId} />;
}

