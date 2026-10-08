import { apiURL, request } from "@/lib/api/api";
import { parseSimulation, watchSimulation, type Simulation } from "@/lib/api/simulation-stream";

export type JobKind = "simulation" | "build";

// Simulations and real builds share one API shape under different collections.
function deploymentJobs(kind: JobKind) {
  const simulated = kind === "simulation";
  const collection = simulated ? "deployment-simulations" : "deployments";
  const path = (slug: string, app: string) => `/api/v1/workspaces/${encodeURIComponent(slug)}/applications/${encodeURIComponent(app)}/${collection}`;
  const parse = (value: unknown) => parseSimulation(value, simulated);
  return {
    path,
    parse,
    async list(slug: string, app: string, token: string | null, signal?: AbortSignal) {
      const result = await request<{ deployments: Simulation[] }>(path(slug, app), token, { signal });
      return result.deployments.map(parse);
    },
    async get(slug: string, app: string, id: string, token: string | null, signal?: AbortSignal) {
      return parse(await request(`${path(slug, app)}/${encodeURIComponent(id)}`, token, { signal }));
    },
    async cancel(slug: string, app: string, id: string, token: string | null) {
      return parse(await request(`${path(slug, app)}/${encodeURIComponent(id)}/cancel`, token, { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" }));
    },
    watch(slug: string, app: string, id: string, options: Omit<Parameters<typeof watchSimulation>[0], "url" | "jobId" | "appId" | "simulated">) {
      return watchSimulation({ ...options, simulated, appId: app, jobId: id, url: `${apiURL}${path(slug, app)}/${encodeURIComponent(id)}/events` });
    },
  };
}

const simulationJobs = deploymentJobs("simulation");
const buildJobs = deploymentJobs("build");

export const simulations = {
  ...simulationJobs,
  async start(slug: string, app: string, token: string | null) {
    return simulationJobs.parse(await request(simulationJobs.path(slug, app), token, { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" }));
  },
};

export const builds = {
  ...buildJobs,
  // Uploads a plain .tar of the application; the server caps it at 16 MiB.
  async upload(slug: string, app: string, archive: Blob, token: string | null) {
    return buildJobs.parse(await request(buildJobs.path(slug, app), token, { method: "POST", headers: { "Content-Type": "application/x-tar" }, body: archive }));
  },
};

export const jobsFor = (kind: JobKind) => (kind === "simulation" ? simulations : builds);
