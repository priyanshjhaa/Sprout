export const queryKeys = {
  me: (userId: string | null | undefined) => ["user", userId, "me"] as const,
  workspace: (userId: string | null | undefined, slug: string) => ["user", userId, "workspace", slug] as const,
  apps: (userId: string | null | undefined, slug: string) => ["user", userId, "workspace", slug, "apps"] as const,
  app: (userId: string | null | undefined, slug: string, appId: string) => ["user", userId, "workspace", slug, "app", appId] as const,
  deployments: (userId: string | null | undefined, slug: string, appId: string) =>
    ["user", userId, "workspace", slug, "app", appId, "deployments"] as const,
  logs: (userId: string | null | undefined, slug: string, appId: string) => ["user", userId, "workspace", slug, "app", appId, "logs"] as const,
  environment: (userId: string | null | undefined, slug: string, appId: string) =>
    ["user", userId, "workspace", slug, "app", appId, "environment"] as const,
  access: (userId: string | null | undefined, slug: string, appId: string) => ["user", userId, "workspace", slug, "app", appId, "access"] as const,
  agent: (userId: string | null | undefined, slug: string) => ["user", userId, "workspace", slug, "agent"] as const,
};
