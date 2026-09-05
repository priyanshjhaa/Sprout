export const queryKeys = {
  workspace: (slug: string) => ["workspace", slug] as const,
  apps: (slug: string) => ["workspace", slug, "apps"] as const,
  app: (slug: string, appId: string) => ["workspace", slug, "app", appId] as const,
  deployments: (slug: string, appId: string) =>
    ["workspace", slug, "app", appId, "deployments"] as const,
  logs: (slug: string, appId: string) => ["workspace", slug, "app", appId, "logs"] as const,
  environment: (slug: string, appId: string) =>
    ["workspace", slug, "app", appId, "environment"] as const,
  access: (slug: string, appId: string) => ["workspace", slug, "app", appId, "access"] as const,
  agent: (slug: string) => ["workspace", slug, "agent"] as const,
};

