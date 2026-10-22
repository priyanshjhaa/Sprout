import { WorkspaceSettings } from "@/components/settings/workspace-settings";

export default async function WorkspaceSettingsPage({
  params,
}: {
  params: Promise<{ workspaceSlug: string }>;
}) {
  const { workspaceSlug } = await params;
  return <WorkspaceSettings workspaceSlug={workspaceSlug} />;
}
