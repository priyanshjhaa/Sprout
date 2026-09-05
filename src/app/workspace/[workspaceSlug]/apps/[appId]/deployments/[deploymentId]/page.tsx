import { DeploymentDetail } from "@/components/deployments/deployment-detail";

export default async function DeploymentDetailPage({ params }: { params: Promise<{ workspaceSlug: string; appId: string; deploymentId: string }> }) {
  const { workspaceSlug, appId, deploymentId } = await params;
  return <DeploymentDetail workspaceSlug={workspaceSlug} appId={appId} deploymentId={deploymentId} />;
}

