import { AppsIndex } from "@/components/apps/apps-index";

export default async function AppsPage({
  params,
}: {
  params: Promise<{ workspaceSlug: string }>;
}) {
  const { workspaceSlug } = await params;
  return <AppsIndex workspaceSlug={workspaceSlug} />;
}

