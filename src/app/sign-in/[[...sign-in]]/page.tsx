import { SignIn } from "@clerk/nextjs";
import { auth } from "@clerk/nextjs/server";
import { redirect } from "next/navigation";
import { Brand } from "@/components/brand";

export default async function SignInPage() {
  const { userId } = await auth();
  if (userId) redirect("/start");

  return (
    <main className="auth-page">
      <div className="auth-ambient" />
      <nav className="auth-nav"><Brand /></nav>
      <section className="auth-card auth-card-sign-in">
        <span className="seed-icon" aria-hidden="true"><span /><span /></span>
        <p className="eyebrow">Welcome to Sprout</p>
        <h1>Give your small software somewhere to grow.</h1>
        <p>Sign in to your workspace.</p>
        <div className="auth-clerk">
          <SignIn
            routing="path"
            path="/sign-in"
            appearance={{
              elements: {
                rootBox: { width: "100%" },
                cardBox: { width: "100%" },
                card: { width: "100%" },
              },
            }}
          />
        </div>
      </section>
    </main>
  );
}
