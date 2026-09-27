import { SignIn } from "@clerk/nextjs";
import { auth } from "@clerk/nextjs/server";
import { redirect } from "next/navigation";
import { AuthExperience, authAppearance } from "@/components/auth/auth-experience";

export default async function SignInPage() {
  const { userId } = await auth();
  if (userId) redirect("/start");

  return (
    <AuthExperience mode="sign-in">
      <SignIn routing="path" path="/sign-in" appearance={authAppearance} />
    </AuthExperience>
  );
}
