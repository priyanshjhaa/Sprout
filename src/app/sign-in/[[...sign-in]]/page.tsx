import { SignIn } from "@clerk/nextjs";
import { Brand } from "@/components/brand";

export default function SignInPage() {
  return (
    <main className="auth-page">
      <div className="auth-ambient" />
      <nav className="auth-nav"><Brand /></nav>
      <section className="auth-card">
        <span className="seed-icon" aria-hidden="true"><span /><span /></span>
        <p className="eyebrow">Welcome to Sprout</p>
        <h1>Give your small software somewhere to grow.</h1>
        <p>Sign in to your workspace.</p>
        <SignIn routing="path" path="/sign-in" fallbackRedirectUrl="/start" />
      </section>
    </main>
  );
}
