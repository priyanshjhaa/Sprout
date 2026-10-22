import { ArrowRight } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

type Variant = "dark" | "light" | "ghost";

export function ActionLink({
  href,
  children,
  variant = "dark",
  small = false,
  arrow = false,
}: {
  href: string;
  children: ReactNode;
  variant?: Variant;
  small?: boolean;
  arrow?: boolean;
}) {
  const className = `lp-btn lp-btn--${variant}${small ? " lp-btn--small" : ""}`;
  return (
    <Link className={className} href={href}>
      {children}
      {arrow && <ArrowRight size={16} aria-hidden="true" />}
    </Link>
  );
}
