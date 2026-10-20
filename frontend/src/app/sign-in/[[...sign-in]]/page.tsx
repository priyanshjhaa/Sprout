import { SignIn } from "@clerk/nextjs";
import { auth } from "@clerk/nextjs/server";
import { redirect } from "next/navigation";
import { AuthExperience, authAppearance } from "@/components/auth/auth-experience";

export default async function SignInPage({ searchParams }: { searchParams: Promise<{ invitation?: string }> }) {
  const invitation = (await searchParams).invitation === "1";
  const destination = invitation ? "/invitations/accept" : "/start";
  const { userId } = await auth();
  if (userId) redirect(destination);

  return (
    <AuthExperience mode="sign-in">
      <SignIn routing="path" path="/sign-in" appearance={authAppearance} forceRedirectUrl={destination} signUpForceRedirectUrl={destination} signUpUrl={invitation ? "/sign-up?invitation=1" : "/sign-up"} />
    </AuthExperience>
  );
}
