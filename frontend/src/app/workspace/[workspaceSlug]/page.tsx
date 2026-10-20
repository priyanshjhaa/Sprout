import { WorkspaceHome } from "@/components/dashboard/workspace-home";

export default async function WorkspacePage({
  params,
}: {
  params: Promise<{ workspaceSlug: string }>;
}) {
  const { workspaceSlug } = await params;
  return <WorkspaceHome workspaceSlug={workspaceSlug} />;
}
