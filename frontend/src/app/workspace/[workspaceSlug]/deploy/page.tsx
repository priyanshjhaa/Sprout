import { DeployFlow } from "@/components/deploy/deploy-flow";

export default async function DeployPage({
  params,
}: {
  params: Promise<{ workspaceSlug: string }>;
}) {
  const { workspaceSlug } = await params;
  return <DeployFlow workspaceSlug={workspaceSlug} />;
}
