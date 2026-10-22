import { SignIn } from "@clerk/nextjs";
import { auth } from "@clerk/nextjs/server";
import { redirect } from "next/navigation";
import { AuthShell } from "@/components/auth/auth-shell";
import { authAppearance } from "@/components/auth/clerk-appearance";

export default async function SignInPage({ searchParams }: { searchParams: Promise<{ invitation?: string }> }) {
  const invitation = (await searchParams).invitation === "1";
  const destination = invitation ? "/invitations/accept" : "/start";
  const { userId } = await auth();
  if (userId) redirect(destination);

  return (
    <AuthShell mode="sign-in" invitation={invitation}>
      <SignIn routing="path" path="/sign-in" appearance={authAppearance} forceRedirectUrl={destination} signUpForceRedirectUrl={destination} signUpUrl={invitation ? "/sign-up?invitation=1" : "/sign-up"} />
    </AuthShell>
  );
}
