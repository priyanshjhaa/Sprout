"use client";

import { useAuth } from "@clerk/nextjs";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { api } from "@/lib/api/api";
import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "@/lib/query/keys";

export function StartWorkspace() {
  const router = useRouter();
  const { getToken, isLoaded, isSignedIn, userId } = useAuth();
  const { data, isError, refetch } = useQuery({
    queryKey: queryKeys.me(userId),
    queryFn: async () => api.getMe(await getToken()),
    enabled: isLoaded && isSignedIn,
  });

  useEffect(() => {
    if (data) router.replace(`/workspace/${data.workspace.slug}/apps`);
  }, [data, router]);

  return (
    <main className="auth-page">
      <section className="auth-card">
        <p className="eyebrow">Your workspace</p>
        <h1>{isError ? "We couldn't open your workspace." : "Getting your workspace ready."}</h1>
        {isError ? (
          <button className="button button-primary" type="button" onClick={() => refetch()}>Try again</button>
        ) : (
          <p>One moment while Sprout checks your account.</p>
        )}
      </section>
    </main>
  );
}
