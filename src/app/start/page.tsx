import { auth } from "@clerk/nextjs/server";
import { redirect } from "next/navigation";
import { StartWorkspace } from "@/components/dashboard/start-workspace";

export default async function StartPage() {
  const { userId } = await auth();
  if (!userId) redirect("/sign-in");
  return <StartWorkspace />;
}
