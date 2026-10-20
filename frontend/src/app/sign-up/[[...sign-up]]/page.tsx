import { SignUp } from "@clerk/nextjs";
import { auth } from "@clerk/nextjs/server";
import { redirect } from "next/navigation";
import { AuthExperience, authAppearance } from "@/components/auth/auth-experience";

export default async function SignUpPage({ searchParams }: { searchParams: Promise<{ invitation?: string }> }) {
  const invitation = (await searchParams).invitation === "1";
  const destination = invitation ? "/invitations/accept" : "/start";
  const { userId } = await auth();
  if (userId) redirect(destination);

  return (
    <AuthExperience mode="sign-up">
      <SignUp routing="path" path="/sign-up" appearance={authAppearance} forceRedirectUrl={destination} signInForceRedirectUrl={destination} signInUrl={invitation ? "/sign-in?invitation=1" : "/sign-in"} />
    </AuthExperience>
  );
}
