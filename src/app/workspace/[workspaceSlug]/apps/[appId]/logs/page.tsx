import { LogViewer } from "@/components/logs/log-viewer";

export default async function LogsPage({ params }: { params: Promise<{ workspaceSlug: string; appId: string }> }) {
  const { workspaceSlug, appId } = await params;
  return <LogViewer workspaceSlug={workspaceSlug} appId={appId} />;
}

