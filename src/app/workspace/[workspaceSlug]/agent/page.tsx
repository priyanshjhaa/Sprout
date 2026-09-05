import { AgentWorkspace } from "@/components/agent/agent-workspace";

export default async function AgentPage({
  params,
}: {
  params: Promise<{ workspaceSlug: string }>;
}) {
  const { workspaceSlug } = await params;
  return <AgentWorkspace workspaceSlug={workspaceSlug} />;
}

