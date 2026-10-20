"use client";

import { useAuth } from "@clerk/nextjs";
import { createContext, useContext, type ReactNode } from "react";

// The workspace layout verifies the session on the server and passes the user's
// id down. Clerk's browser script needs a moment to load, and until it does
// useAuth() has no user id, so cache keys built from it would miss the cache
// that was restored after a refresh. Tokens still come only from Clerk.
const SessionUserContext = createContext<string | null>(null);

export function SessionUser({ id, children }: { id: string; children: ReactNode }) {
  return <SessionUserContext.Provider value={id}>{children}</SessionUserContext.Provider>;
}

export function useSessionAuth() {
  const auth = useAuth();
  const hinted = useContext(SessionUserContext);
  return { ...auth, userId: auth.userId ?? (auth.isLoaded ? null : hinted) };
}
