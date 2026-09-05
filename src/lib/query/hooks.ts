"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api/mock-api";
import { queryKeys } from "@/lib/query/keys";

export const useWorkspace = (slug: string) =>
  useQuery({ queryKey: queryKeys.workspace(slug), queryFn: api.getWorkspace });

export const useApplications = (slug: string) =>
  useQuery({ queryKey: queryKeys.apps(slug), queryFn: api.getApplications });

export const useApplication = (slug: string, appId: string) =>
  useQuery({ queryKey: queryKeys.app(slug, appId), queryFn: () => api.getApplication(appId) });

export const useDeployments = (slug: string, appId: string) =>
  useQuery({ queryKey: queryKeys.deployments(slug, appId), queryFn: () => api.getDeployments(appId) });

export const useLogs = (slug: string, appId: string) =>
  useQuery({ queryKey: queryKeys.logs(slug, appId), queryFn: api.getLogs });

export const useEnvironment = (slug: string, appId: string) =>
  useQuery({ queryKey: queryKeys.environment(slug, appId), queryFn: api.getEnvironment });

export const useAccess = (slug: string, appId: string) =>
  useQuery({ queryKey: queryKeys.access(slug, appId), queryFn: api.getMembers });

export const useAgentEvents = (slug: string) =>
  useQuery({ queryKey: queryKeys.agent(slug), queryFn: api.getAgentEvents });

