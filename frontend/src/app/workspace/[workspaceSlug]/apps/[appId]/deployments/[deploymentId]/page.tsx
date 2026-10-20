import { DeploymentDetail } from "@/components/deployments/deployment-detail";

export default async function DeploymentDetailPage({ params, searchParams }: {
  params: Promise<{ workspaceSlug: string; appId: string; deploymentId: string }>;
  searchParams: Promise<{ kind?: string }>;
}) {
  const { workspaceSlug, appId, deploymentId } = await params;
  const kind = (await searchParams).kind === "build" ? "build" : "simulation";
  return <DeploymentDetail workspaceSlug={workspaceSlug} appId={appId} deploymentId={deploymentId} kind={kind} />;
}
