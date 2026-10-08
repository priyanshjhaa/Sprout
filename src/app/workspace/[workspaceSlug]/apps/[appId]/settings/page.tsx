import { ApplicationSettings } from "@/components/settings/application-settings";

export default async function ApplicationSettingsPage({ params }: { params: Promise<{ workspaceSlug: string; appId: string }> }) {
  const { workspaceSlug, appId } = await params;
  return <ApplicationSettings workspaceSlug={workspaceSlug} appId={appId} />;
}
