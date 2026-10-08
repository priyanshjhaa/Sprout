export const queryKeys = {
  workspaces: (userId: string | null | undefined) => ["user", userId, "workspaces"] as const,
  invitations: (userId: string | null | undefined, slug: string) => ["user", userId, "workspace", slug, "invitations"] as const,
  me: (userId: string | null | undefined) => ["user", userId, "me"] as const,
  workspace: (userId: string | null | undefined, slug: string) => ["user", userId, "workspace", slug] as const,
  apps: (userId: string | null | undefined, slug: string) => ["user", userId, "workspace", slug, "apps"] as const,
  app: (userId: string | null | undefined, slug: string, appId: string) => ["user", userId, "workspace", slug, "app", appId] as const,
  deployments: (userId: string | null | undefined, slug: string, appId: string) =>
    ["user", userId, "workspace", slug, "app", appId, "deployments"] as const,
  simulation: (userId: string | null | undefined, slug: string, appId: string, id: string) =>
    ["user", userId, "workspace", slug, "app", appId, "deployments", id] as const,
  builds: (userId: string | null | undefined, slug: string, appId: string) =>
    ["user", userId, "workspace", slug, "app", appId, "builds"] as const,
  build: (userId: string | null | undefined, slug: string, appId: string, id: string) =>
    ["user", userId, "workspace", slug, "app", appId, "builds", id] as const,
  capabilities: (userId: string | null | undefined) => ["user", userId, "capabilities"] as const,
  access: (userId: string | null | undefined, slug: string, appId: string) => ["user", userId, "workspace", slug, "app", appId, "access"] as const,
  members: (userId: string | null | undefined, slug: string) => ["user", userId, "workspace", slug, "members"] as const,
};
