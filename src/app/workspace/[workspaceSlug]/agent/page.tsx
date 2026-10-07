import { redirect } from "next/navigation";

// The Agent page became Deploy; keep old links working.
export default async function AgentPage({
  params,
}: {
  params: Promise<{ workspaceSlug: string }>;
}) {
  const { workspaceSlug } = await params;
  redirect(`/workspace/${workspaceSlug}/deploy`);
}
