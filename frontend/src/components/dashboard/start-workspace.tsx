"use client";

import { useAuth } from "@clerk/nextjs";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { api } from "@/lib/api/api";
import { useQuery } from "@tanstack/react-query";
import { AuthShell } from "@/components/auth/auth-shell";
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
    if (data) router.replace(`/workspace/${data.workspace.slug}`);
  }, [data, router]);

  return (
    <AuthShell mode="start">
      <div className="auth-status">
        {isError ? (
          <>
            <h2>We couldn&apos;t open your workspace.</h2>
            <p>Sprout could not reach your account just now. You can safely try again.</p>
            <button className="lp-btn lp-btn--dark" type="button" onClick={() => refetch()}>Try again</button>
          </>
        ) : (
          <>
            <span className="auth-spinner" aria-hidden="true" />
            <p role="status">One moment while Sprout checks your account.</p>
          </>
        )}
      </div>
    </AuthShell>
  );
}
