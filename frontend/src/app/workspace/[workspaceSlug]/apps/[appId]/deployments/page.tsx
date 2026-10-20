import { DeploymentsView } from "@/components/deployments/deployments-view";

export default async function DeploymentsPage({ params }: { params: Promise<{ workspaceSlug: string; appId: string }> }) {
  const { workspaceSlug, appId } = await params;
  return <DeploymentsView workspaceSlug={workspaceSlug} appId={appId} />;
}

