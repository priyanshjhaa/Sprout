import Link from "next/link";

export function Brand({ compact = false }: { compact?: boolean }) {
  return (
    <Link className="brand" href="/" aria-label="Sprout home">
      <span className="brand-mark" aria-hidden="true">
        <span />
        <span />
      </span>
      {!compact && <span>sprout</span>}
    </Link>
  );
}

