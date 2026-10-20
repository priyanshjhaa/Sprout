"use client";

import { useAuth } from "@clerk/nextjs";
import { createSyncStoragePersister } from "@tanstack/query-sync-storage-persister";
import { QueryClient, type Query } from "@tanstack/react-query";
import { PersistQueryClientProvider } from "@tanstack/react-query-persist-client";
import { useEffect, useState, type ReactNode } from "react";

const DAY = 24 * 60 * 60 * 1000;
const STORAGE_KEY = "sprout-query-cache";

// Only read-mostly data is kept across refreshes. Deployments and builds change
// while you watch, and invitations and access are sensitive, so they always
// load fresh. Every key starts with the user id, so one account's cache can
// never be served to another.
function persistable(query: Query): boolean {
  if (query.state.status !== "success") return false;
  const [scope, , kind, , detail, appId, extra] = query.queryKey as string[];
  if (scope !== "user") return false;
  if (["workspaces", "me", "capabilities"].includes(kind)) return true;
  if (kind !== "workspace") return false;
  if (detail === undefined || detail === "apps" || detail === "members") return true;
  return detail === "app" && appId !== undefined && extra === undefined;
}

export function Providers({ children }: { children: ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          // Data stays fresh for five minutes, and is kept long enough to be
          // restored after a refresh. Mutations invalidate what they change.
          queries: { staleTime: 5 * 60_000, gcTime: DAY, refetchOnWindowFocus: false },
        },
      }),
  );
  const [persister] = useState(() =>
    createSyncStoragePersister({
      storage: typeof window === "undefined" ? undefined : window.sessionStorage,
      key: STORAGE_KEY,
      throttleTime: 1000,
    }),
  );
  const { isLoaded, userId } = useAuth();

  // Signing out clears everything, in memory and in the tab's storage.
  useEffect(() => {
    if (isLoaded && !userId) {
      queryClient.clear();
      window.sessionStorage.removeItem(STORAGE_KEY);
    }
  }, [isLoaded, userId, queryClient]);

  return (
    <PersistQueryClientProvider
      client={queryClient}
      persistOptions={{ persister, maxAge: DAY, buster: "v1", dehydrateOptions: { shouldDehydrateQuery: persistable } }}
    >
      {children}
    </PersistQueryClientProvider>
  );
}
