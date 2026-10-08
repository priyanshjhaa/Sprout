// A deployment job: an explicit simulation or a real build. Both share one shape.
export interface Simulation {
  id: string;
  applicationId: string;
  simulated: boolean;
  status: "queued" | "building" | "succeeded" | "failed" | "cancelled";
  failureCode: string | null;
  createdAt: string;
  startedAt: string | null;
  finishedAt: string | null;
  stages: Array<{ name: "source" | "build" | "package"; status: "pending" | "running" | "succeeded" | "failed" | "skipped"; failureCode: string | null }>;
}

export const isActiveSimulation = (job: Simulation) => job.status === "queued" || job.status === "building";

export function reconcileSimulation(old: Simulation | undefined, incoming: Simulation): Simulation {
  return old && !isActiveSimulation(old) ? old : incoming;
}

// The caller states which kind it expects, so a simulation view can never show
// a real build (or the reverse), whatever the server returns.
export function parseSimulation(value: unknown, simulated = true): Simulation {
  const job = value as Simulation;
  const nullableString = (field: unknown) => field === null || typeof field === "string";
  if (!job || typeof job.id !== "string" || typeof job.applicationId !== "string" || job.simulated !== simulated ||
    !["queued", "building", "succeeded", "failed", "cancelled"].includes(job.status) ||
    typeof job.createdAt !== "string" || !Number.isFinite(Date.parse(job.createdAt)) ||
    !nullableString(job.startedAt) || !nullableString(job.finishedAt) || !nullableString(job.failureCode) ||
    !Array.isArray(job.stages) || job.stages.length > 3 || new Set(job.stages.map((stage) => stage?.name)).size !== job.stages.length ||
    !job.stages.every((stage) => stage && ["source", "build", "package"].includes(stage.name) &&
      ["pending", "running", "succeeded", "failed", "skipped"].includes(stage.status) && nullableString(stage.failureCode))) {
    throw new Error("Invalid simulation response.");
  }
  return {
    id: job.id, applicationId: job.applicationId, simulated, status: job.status,
    createdAt: job.createdAt, startedAt: job.startedAt, finishedAt: job.finishedAt, failureCode: job.failureCode,
    stages: job.stages.map(({ name, status, failureCode }) => ({ name, status, failureCode })),
  };
}

class StreamFailure extends Error {
  retryable: boolean;
  constructor(message: string, retryable = false) { super(message); this.retryable = retryable; }
}

// One bounded frame at a time, including frames split across network chunks.
export async function readProgress(
  body: ReadableStream<Uint8Array>, signal: AbortSignal,
  receive: (event: string, data: string, id: string) => boolean,
): Promise<void> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  const abort = () => { void reader.cancel().catch(() => {}); };
  signal.addEventListener("abort", abort, { once: true });
  try {
    while (!signal.aborted) {
      const { done, value } = await reader.read();
      if (done) return;
      buffer = (buffer + decoder.decode(value, { stream: true })).replace(/\r\n/g, "\n");
      let end: number;
      while ((end = buffer.indexOf("\n\n")) !== -1) {
        if (end > 65_536) throw new StreamFailure("Progress update exceeded its size limit.");
        const frame = buffer.slice(0, end); buffer = buffer.slice(end + 2);
        let event = "", id = ""; const data: string[] = [];
        for (const line of frame.split("\n")) {
          const colon = line.indexOf(":");
          const field = colon < 0 ? line : line.slice(0, colon);
          const content = colon < 0 ? "" : line.slice(colon + 1).replace(/^ /, "");
          if (field === "event") event = content;
          if (field === "id") id = content;
          if (field === "data") data.push(content);
        }
        if (!signal.aborted && event && receive(event, data.join("\n"), id)) return;
      }
      if (buffer.length > 65_536) throw new StreamFailure("Progress update exceeded its size limit.");
    }
  } finally {
    signal.removeEventListener("abort", abort);
    await reader.cancel().catch(() => {});
    reader.releaseLock();
  }
}

function wait(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    if (signal.aborted) return resolve();
    const finish = () => { clearTimeout(timer); signal.removeEventListener("abort", finish); resolve(); };
    const timer = setTimeout(finish, ms);
    signal.addEventListener("abort", finish, { once: true });
  });
}

export async function watchSimulation(options: {
  url: string; jobId: string; appId: string; signal: AbortSignal;
  simulated?: boolean;
  getToken: () => Promise<string | null>;
  onProgress: (job: Simulation) => void;
  onState: (state: "connecting" | "live" | "reconnecting" | "complete") => void;
  fetcher?: typeof fetch;
  delay?: typeof wait;
}): Promise<void> {
  let cursor = "", failures = 0;
  const { signal } = options;
  while (!signal.aborted) {
    let complete = false;
    const connection = new AbortController();
    const abort = () => connection.abort();
    signal.addEventListener("abort", abort, { once: true });
    const timeout = setTimeout(abort, 35_000);
    try {
      options.onState(failures ? "reconnecting" : "connecting");
      const token = await options.getToken();
      if (signal.aborted) return;
      if (!token) throw new StreamFailure("Sign in again to view progress.");
      const response = await (options.fetcher ?? fetch)(options.url, {
        signal: connection.signal, cache: "no-store",
        headers: { Accept: "text/event-stream", Authorization: `Bearer ${token}`, ...(cursor ? { "Last-Event-ID": cursor } : {}) },
      });
      if (!response.ok) {
        await response.body?.cancel();
        throw new StreamFailure(response.status === 403 || response.status === 404 ? "This simulation is no longer accessible." : "Progress is temporarily unavailable. Try again.", response.status >= 500 || response.status === 429 || response.status === 401);
      }
      if (!response.body || !response.headers.get("Content-Type")?.startsWith("text/event-stream")) {
        await response.body?.cancel();
        throw new StreamFailure("Unexpected progress response.");
      }
      await readProgress(response.body, connection.signal, (event, data, id) => {
        if (signal.aborted) return true;
        if (event === "unavailable") throw new StreamFailure("Progress access changed or the service is unavailable. Retry to check again.");
        if (event === "complete") { complete = true; options.onState("complete"); return true; }
        if (event === "progress") {
          const job = parseSimulation(JSON.parse(data), options.simulated ?? true);
          if (job.id !== options.jobId || job.applicationId !== options.appId || !/^[a-f0-9]{64}$/.test(id)) throw new StreamFailure("Invalid progress response.");
          cursor = id; failures = 0;
          options.onProgress(job); options.onState("live");
          // A terminal snapshot is sufficient even if the final frame is lost.
          if (!isActiveSimulation(job)) { complete = true; options.onState("complete"); return true; }
        }
        return false;
      });
      if (complete || signal.aborted) return;
    } catch (error) {
      if (signal.aborted) return;
      if (error instanceof StreamFailure && !error.retryable) throw error;
    } finally {
      clearTimeout(timeout); signal.removeEventListener("abort", abort); connection.abort();
    }
    if (++failures > 5) throw new Error("Live progress disconnected. Retry to reconnect.");
    options.onState("reconnecting");
    await (options.delay ?? wait)(Math.min(16_000, 1000 * 2 ** (failures - 1)), signal);
  }
}
