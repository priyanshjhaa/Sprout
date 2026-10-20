// Presentation helpers for personal access tokens. Pure functions, so they are
// testable without a browser.

const DAY = 24 * 60 * 60 * 1000;

export const expiryChoices = [
  { days: 30, label: "30 days" },
  { days: 90, label: "90 days (recommended)" },
  { days: 365, label: "1 year" },
] as const;

// How a token's expiry reads in a list: relative, and plain when it is gone.
export function describeExpiry(expiresAt: string, now: number = Date.now()): string {
  const remaining = Date.parse(expiresAt) - now;
  if (!Number.isFinite(remaining)) return "Expiry unknown";
  if (remaining <= 0) return "Expired";
  if (remaining < DAY) return "Expires within a day";
  const days = Math.floor(remaining / DAY);
  return days === 1 ? "Expires in 1 day" : `Expires in ${days} days`;
}

export function describeLastUsed(lastUsedAt: string | null, now: number = Date.now()): string {
  if (!lastUsedAt) return "Never used";
  const elapsed = now - Date.parse(lastUsedAt);
  if (!Number.isFinite(elapsed)) return "Used";
  if (elapsed < 60_000) return "Used just now";
  if (elapsed < 60 * 60_000) return `Used ${Math.floor(elapsed / 60_000)} min ago`;
  if (elapsed < DAY) return `Used ${Math.floor(elapsed / (60 * 60_000))} hr ago`;
  const days = Math.floor(elapsed / DAY);
  return days === 1 ? "Used yesterday" : `Used ${days} days ago`;
}

// The only part of a token the dashboard ever shows after creation.
export const displayHint = (hint: string) => `sprout_pat_…${hint}`;
