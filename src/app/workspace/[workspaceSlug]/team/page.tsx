import { TeamView } from "@/components/access/team-view";

export default async function TeamPage({ params }: { params: Promise<{ workspaceSlug: string }> }) {
  const { workspaceSlug } = await params;
  return <TeamView workspaceSlug={workspaceSlug} />;
}
