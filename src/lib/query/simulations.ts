"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { request } from "@/lib/api/api";
import { builds, jobsFor, simulations, type JobKind } from "@/lib/api/simulations";
import { isActiveSimulation, reconcileSimulation, type Simulation } from "@/lib/api/simulation-stream";
import { queryKeys } from "@/lib/query/keys";

export function useSimulationActions(slug: string, appId: string) {
  const { getToken, userId } = useAuth();
  const client = useQueryClient();
  return useMutation({
    mutationFn: async (id?: string) => id
      ? simulations.cancel(slug, appId, id, await getToken())
      : simulations.start(slug, appId, await getToken()),
    retry: false, // Never automatically repeat an ambiguous POST.
    onSuccess: async (job) => {
      const key = queryKeys.simulation(userId, slug, appId, job.id);
      await client.cancelQueries({ queryKey: key, exact: true });
      client.setQueryData(key, job);
    },
    onSettled: () => client.invalidateQueries({ queryKey: queryKeys.deployments(userId, slug, appId) }),
  });
}

// Which optional features the API offers; local builds are off unless enabled.
export function useCapabilities() {
  const { getToken, isLoaded, isSignedIn, userId } = useAuth();
  return useQuery({
    queryKey: queryKeys.capabilities(userId),
    queryFn: async () => request<{ localBuilds: boolean }>("/api/v1/capabilities", await getToken()),
    enabled: isLoaded && isSignedIn,
    staleTime: Infinity,
  });
}

export function useBuilds(slug: string, appId: string, enabled: boolean) {
  const { getToken, isLoaded, isSignedIn, userId } = useAuth();
  return useQuery({
    queryKey: queryKeys.builds(userId, slug, appId),
    queryFn: async ({ signal }) => builds.list(slug, appId, await getToken(), signal),
    enabled: enabled && isLoaded && isSignedIn,
    retry: false,
    refetchInterval: (query) => !query.state.error && query.state.data?.some(isActiveSimulation) ? 5000 : false,
  });
}

// Upload an archive to build, or cancel a build by id.
export function useBuildActions(slug: string, appId: string) {
  const { getToken, userId } = useAuth();
  const client = useQueryClient();
  return useMutation({
    mutationFn: async (input: { upload: Blob } | { cancel: string }) => "upload" in input
      ? builds.upload(slug, appId, input.upload, await getToken())
      : builds.cancel(slug, appId, input.cancel, await getToken()),
    retry: false, // Never automatically repeat an ambiguous POST.
    onSuccess: async (job) => {
      const key = queryKeys.build(userId, slug, appId, job.id);
      await client.cancelQueries({ queryKey: key, exact: true });
      client.setQueryData(key, job);
    },
    onSettled: () => client.invalidateQueries({ queryKey: queryKeys.builds(userId, slug, appId) }),
  });
}

export function useSimulation(slug: string, appId: string, id: string, kind: JobKind = "simulation") {
  const { getToken, userId, isLoaded, isSignedIn } = useAuth();
  const client = useQueryClient();
  const [attempt, setAttempt] = useState(0);
  const identity = JSON.stringify([userId, slug, appId, id, kind, attempt]);
  const [connection, setConnection] = useState<{ identity: string; state: string; error?: string }>();
  const query = useQuery({
    queryKey: kind === "simulation" ? queryKeys.simulation(userId, slug, appId, id) : queryKeys.build(userId, slug, appId, id),
    queryFn: async ({ signal }) => jobsFor(kind).get(slug, appId, id, await getToken(), signal),
    enabled: isLoaded && isSignedIn,
    retry: false,
    refetchOnWindowFocus: false,
  });
  // Finish snapshot fetches before listening so an older GET cannot overwrite SSE.
  const active = !!query.data && isActiveSimulation(query.data) && !query.isError && !query.isFetching;
  useEffect(() => {
    if (!isLoaded || !isSignedIn || !active) return;
    const controller = new AbortController();
    // Keys are built here, not above, so the effect's dependencies stay stable.
    const key = kind === "simulation" ? queryKeys.simulation(userId, slug, appId, id) : queryKeys.build(userId, slug, appId, id);
    const listKey = kind === "simulation" ? queryKeys.deployments(userId, slug, appId) : queryKeys.builds(userId, slug, appId);
    void jobsFor(kind).watch(slug, appId, id, {
      signal: controller.signal,
      getToken: () => getToken({ skipCache: true }),
      onProgress: (job) => {
        // A cancelled/finished job must not regress from an in-flight frame.
        client.setQueryData<Simulation>(key, (old) => reconcileSimulation(old, job));
      },
      onState: (state) => {
        if (controller.signal.aborted) return;
        setConnection({ identity, state });
        if (state === "complete") void client.invalidateQueries({ queryKey: listKey, exact: true });
      },
    }).catch(() => {
      if (controller.signal.aborted) return;
      setConnection({ identity, state: "disconnected", error: "Live progress is unavailable. Retry to check your access and reconnect." });
      void client.invalidateQueries({ queryKey: listKey, exact: true });
    });
    return () => controller.abort();
  }, [active, appId, client, getToken, id, identity, isLoaded, isSignedIn, kind, slug, userId]);

  return {
    ...query,
    streamState: connection?.identity === identity ? connection.state : "connecting",
    streamError: connection?.identity === identity ? connection.error : undefined,
    reconnect: async () => {
      const result = await query.refetch();
      if (!result.isError) setAttempt((value) => value + 1);
    },
  };
}
