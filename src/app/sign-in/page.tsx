import { ArrowRight, GitBranch } from "lucide-react";
import Link from "next/link";
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
        <p>Enter the product preview with a demo workspace. GitHub authentication will arrive with the backend.</p>
        <Link className="button button-primary button-wide" href="/workspace/acme/agent">
          <GitBranch size={17} /> Continue with demo workspace <ArrowRight size={16} />
        </Link>
        <small>This preview does not connect to your GitHub account.</small>
      </section>
    </main>
  );
}
