import { SignUp } from "@clerk/nextjs";
import { auth } from "@clerk/nextjs/server";
import { redirect } from "next/navigation";
import { AuthShell } from "@/components/auth/auth-shell";
import { authAppearance } from "@/components/auth/clerk-appearance";

export default async function SignUpPage({ searchParams }: { searchParams: Promise<{ invitation?: string }> }) {
  const invitation = (await searchParams).invitation === "1";
  const destination = invitation ? "/invitations/accept" : "/start";
  const { userId } = await auth();
  if (userId) redirect(destination);

  return (
    <AuthShell mode="sign-up" invitation={invitation}>
      <SignUp routing="path" path="/sign-up" appearance={authAppearance} forceRedirectUrl={destination} signInForceRedirectUrl={destination} signInUrl={invitation ? "/sign-in?invitation=1" : "/sign-in"} />
    </AuthShell>
  );
}
