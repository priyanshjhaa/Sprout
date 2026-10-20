import { ApplicationOverview } from "@/components/apps/application-overview";

export default async function ApplicationPage({
  params,
}: {
  params: Promise<{ workspaceSlug: string; appId: string }>;
}) {
  const { workspaceSlug, appId } = await params;
  return <ApplicationOverview workspaceSlug={workspaceSlug} appId={appId} />;
}

