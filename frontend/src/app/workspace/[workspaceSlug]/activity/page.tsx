import { redirect } from "next/navigation";

// Not part of the product yet; keep old links landing somewhere useful.
export default async function WorkspacePage({
  params,
}: {
  params: Promise<{ workspaceSlug: string }>;
}) {
  const { workspaceSlug } = await params;
  redirect(`/workspace/${workspaceSlug}/apps`);
}
