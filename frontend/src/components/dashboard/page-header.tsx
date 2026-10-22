import type { ReactNode } from "react";

// The heading every workspace page opens with: a mono eyebrow, a title, a line of context, and actions.
export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
}: {
  eyebrow: string;
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <header className="dash-page-head">
      <div>
        <p className="dash-hero-eyebrow">{eyebrow}</p>
        <h1>{title}</h1>
        {description && <p className="dash-page-description">{description}</p>}
      </div>
      {actions && <div className="dash-page-actions">{actions}</div>}
    </header>
  );
}
