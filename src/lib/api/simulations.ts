import { apiURL, request } from "@/lib/api/api";
import { parseSimulation, watchSimulation, type Simulation } from "@/lib/api/simulation-stream";

const path = (slug: string, app: string) => `/api/v1/workspaces/${encodeURIComponent(slug)}/applications/${encodeURIComponent(app)}/deployment-simulations`;
export const simulations = {
  async list(slug: string, app: string, token: string | null, signal?: AbortSignal) {
    const result = await request<{ deployments: Simulation[] }>(path(slug, app), token, { signal });
    return result.deployments.map(parseSimulation);
  },
  async get(slug: string, app: string, id: string, token: string | null, signal?: AbortSignal) {
    return parseSimulation(await request(`${path(slug, app)}/${encodeURIComponent(id)}`, token, { signal }));
  },
  async start(slug: string, app: string, token: string | null) {
    return parseSimulation(await request(path(slug, app), token, { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" }));
  },
  async cancel(slug: string, app: string, id: string, token: string | null) {
    return parseSimulation(await request(`${path(slug, app)}/${encodeURIComponent(id)}/cancel`, token, { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" }));
  },
  watch(slug: string, app: string, id: string, options: Omit<Parameters<typeof watchSimulation>[0], "url" | "jobId" | "appId">) {
    return watchSimulation({ ...options, appId: app, jobId: id, url: `${apiURL}${path(slug, app)}/${encodeURIComponent(id)}/events` });
  },
};
