"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAuth, useClerk } from "@clerk/nextjs";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api/api";
import { queryKeys } from "@/lib/query/keys";

const storageKey = "sprout.pending-invitation";

export function InvitationAccept() {
  const { isLoaded, isSignedIn, getToken, userId } = useAuth();
  const { signOut } = useClerk();
  const router = useRouter();
  const client = useQueryClient();
  const [invitation, setInvitation] = useState<string | null>(null);
  const [storageError, setStorageError] = useState(false);
  useEffect(() => {
    // URL fragments are not sent to servers or included in HTTP referrers.
    // Session storage preserves this one token through same-tab OAuth only.
    const readInvitation = () => {
      const token = window.location.hash.slice(1);
      try {
        if (token) {
          sessionStorage.removeItem(storageKey);
          if (/^[A-Za-z0-9_-]{43}$/.test(token)) sessionStorage.setItem(storageKey, token);
        }
        setInvitation(sessionStorage.getItem(storageKey) ?? "");
      } catch { setStorageError(true); setInvitation(""); }
      window.history.replaceState(null, "", window.location.pathname);
    };
    // Initialize from browser-only external state, then handle another link in this tab.
    readInvitation();
    window.addEventListener("hashchange", readInvitation);
    return () => window.removeEventListener("hashchange", readInvitation);
  }, []);
  const accept = useMutation({
    gcTime: 0,
    mutationFn: async () => api.acceptInvitation(invitation ?? "", await getToken()),
    onSuccess: async (workspace) => {
      sessionStorage.removeItem(storageKey);
      setInvitation("");
      await client.invalidateQueries({ queryKey: queryKeys.workspaces(userId) });
      router.replace(`/workspace/${encodeURIComponent(workspace.slug)}/apps`);
    },
  });

  if (!isLoaded || invitation === null) return <p role="status">Opening invitation…</p>;
  return <section className="invitation-accept">
    <p className="eyebrow">A place on the team</p><h1>You’re invited.</h1>
    <p>Sign in with the verified primary email your workspace owner invited, then join the workspace.</p>
    {storageError ? <p role="alert">Allow session storage in this browser, then reopen the invitation link.</p> : !invitation ? <p role="status">Open the full invitation link shared by your workspace owner.</p> :
      isSignedIn ? <>
        <button className="button button-primary" disabled={accept.isPending || accept.isSuccess} onClick={() => accept.mutate()}>{accept.isPending ? "Joining…" : accept.isSuccess ? "Opening workspace…" : "Join workspace"}</button>
        <button className="button button-secondary" disabled={accept.isPending} onClick={() => signOut({ redirectUrl: "/sign-in?invitation=1" })}>Use a different account</button>
      </> : <Link className="button button-primary" href="/sign-in?invitation=1">Sign in to join</Link>}
    {accept.isError && <p role="alert">{accept.error.message}</p>}
    <Link href="/start">Go to my workspace</Link>
  </section>;
}
