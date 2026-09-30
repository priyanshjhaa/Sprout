"use client";

import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@clerk/nextjs";
import { api } from "@/lib/api/api";
import { queryKeys } from "@/lib/query/keys";
import { simulations } from "@/lib/api/simulations";
import { isActiveSimulation } from "@/lib/api/simulation-stream";

export const useMe = () => {
  const { getToken, isLoaded, isSignedIn, userId } = useAuth();
  return useQuery({ queryKey: queryKeys.me(userId), queryFn: async () => api.getMe(await getToken()), enabled: isLoaded && isSignedIn });
};

export const useWorkspaces = () => {
  const { getToken, isLoaded, isSignedIn, userId } = useAuth();
  return useQuery({ queryKey: queryKeys.workspaces(userId), queryFn: async () => api.getWorkspaces(await getToken()), enabled: isLoaded && isSignedIn });
};

export const useInvitations = (slug: string, isOwner: boolean) => {
  const { getToken, isLoaded, isSignedIn, userId } = useAuth();
  return useQuery({ queryKey: queryKeys.invitations(userId, slug), queryFn: async () => api.getInvitations(slug, await getToken()), enabled: isLoaded && isSignedIn && isOwner });
};

export const useWorkspace = (slug: string) => {
  const { getToken, isLoaded, isSignedIn, userId } = useAuth();
  return useQuery({
    queryKey: queryKeys.workspace(userId, slug),
    queryFn: async () => api.getWorkspace(slug, await getToken()),
    enabled: isLoaded && isSignedIn,
  });
};

export const useApplications = (slug: string) => {
  const { getToken, isLoaded, isSignedIn, userId } = useAuth();
  return useQuery({
    queryKey: queryKeys.apps(userId, slug),
    queryFn: async () => api.getApplications(slug, await getToken()),
    enabled: isLoaded && isSignedIn,
  });
};

export const useApplication = (slug: string, appId: string) => {
  const { getToken, isLoaded, isSignedIn, userId } = useAuth();
  return useQuery({
    queryKey: queryKeys.app(userId, slug, appId),
    queryFn: async () => api.getApplication(slug, appId, await getToken()),
    enabled: isLoaded && isSignedIn,
  });
};

export const useDeployments = (slug: string, appId: string) => {
  const { getToken, isLoaded, isSignedIn, userId } = useAuth();
  return useQuery({
    queryKey: queryKeys.deployments(userId, slug, appId),
    queryFn: async ({ signal }) => simulations.list(slug, appId, await getToken(), signal),
    enabled: isLoaded && isSignedIn,
    retry: false,
    refetchInterval: (query) => !query.state.error && query.state.data?.some(isActiveSimulation) ? 5000 : false,
  });
};

export const useLogs = (slug: string, appId: string) => {
  const { userId } = useAuth();
  return useQuery({ queryKey: queryKeys.logs(userId, slug, appId), queryFn: api.getLogs });
};

export const useEnvironment = (slug: string, appId: string) => {
  const { userId } = useAuth();
  return useQuery({ queryKey: queryKeys.environment(userId, slug, appId), queryFn: api.getEnvironment });
};

export const useAccess = (slug: string, appId: string) => {
  const { getToken, isLoaded, isSignedIn, userId } = useAuth();
  return useQuery({
    queryKey: queryKeys.access(userId, slug, appId),
    queryFn: async () => api.getApplicationAccess(slug, appId, await getToken()),
    enabled: isLoaded && isSignedIn,
  });
};

export const useWorkspaceMembers = (slug: string) => {
  const { getToken, isLoaded, isSignedIn, userId } = useAuth();
  return useQuery({
    queryKey: queryKeys.members(userId, slug),
    queryFn: async () => api.getWorkspaceMembers(slug, await getToken()),
    enabled: isLoaded && isSignedIn,
  });
};

export const useAgentEvents = (slug: string) => {
  const { userId } = useAuth();
  return useQuery({ queryKey: queryKeys.agent(userId, slug), queryFn: api.getAgentEvents });
};
