import { SignUp } from "@clerk/nextjs";
import { auth } from "@clerk/nextjs/server";
import { redirect } from "next/navigation";
import { AuthExperience, authAppearance } from "@/components/auth/auth-experience";

export default async function SignUpPage() {
  const { userId } = await auth();
  if (userId) redirect("/start");

  return (
    <AuthExperience mode="sign-up">
      <SignUp routing="path" path="/sign-up" appearance={authAppearance} />
    </AuthExperience>
  );
}
