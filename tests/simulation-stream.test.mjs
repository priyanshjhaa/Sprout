import test from "node:test";
import assert from "node:assert/strict";
import { readProgress, watchSimulation, parseSimulation, reconcileSimulation } from "../src/lib/api/simulation-stream.ts";

const job = { id: "job", applicationId: "app", simulated: true, status: "building", createdAt: "2026-09-30T12:00:00Z", failureCode: null, startedAt: null, finishedAt: null, stages: [] };
const cursor = "a".repeat(64);
const frame = (value = job) => `id: ${cursor}\nevent: progress\ndata: ${JSON.stringify(value)}\n\n`;
const body = (chunks) => new ReadableStream({ start(controller) { for (const chunk of chunks) controller.enqueue(new TextEncoder().encode(chunk)); controller.close(); } });
const response = (text) => new Response(body([text]), { headers: { "Content-Type": "text/event-stream" } });
const options = (overrides = {}) => ({ url: "http://localhost/events", appId: "app", jobId: "job", signal: new AbortController().signal, getToken: async () => "test-only-token", onProgress() {}, onState() {}, delay: async () => {}, ...overrides });

test("late active frames cannot undo a persisted cancellation", () => {
  const cancelled = { ...job, status: "cancelled" };
  assert.equal(reconcileSimulation(cancelled, job), cancelled);
  assert.equal(reconcileSimulation(job, cancelled), cancelled);
});

test("parses split CRLF, multiline data and ignores comments/retry", async () => {
  const events = [];
  const text = ": heartbeat\r\n\r\nretry: 1000\r\n\r\nevent: progress\r\nid: abc\r\ndata: first\r\ndata: second\r\n\r\n";
  await readProgress(body([...text]), new AbortController().signal, (...event) => { events.push(event); return false; });
  assert.deepEqual(events, [["progress", "first\nsecond", "abc"]]);
});

test("bounds incomplete and complete frames", async () => {
  for (const suffix of ["", "\n\n"]) {
    await assert.rejects(readProgress(body(["x".repeat(70_000) + suffix]), new AbortController().signal, () => false), /size limit/);
  }
});

test("reconnect gets a fresh token, sends cursor and stops at terminal progress", async () => {
  let calls = 0, tokens = 0;
  const snapshots = [];
  await watchSimulation(options({
    getToken: async () => `test-only-${++tokens}`,
    onProgress: (value) => snapshots.push(value.status),
    fetcher: async (url, init) => {
      assert.equal(url.includes("token"), false);
      assert.equal(init.headers.Authorization, `Bearer test-only-${tokens}`);
      assert.equal(init.headers["Last-Event-ID"], calls ? cursor : undefined);
      return response(frame(++calls === 1 ? job : { ...job, status: "succeeded" }));
    },
  }));
  assert.equal(calls, 2); assert.equal(tokens, 2);
  assert.deepEqual(snapshots, ["building", "succeeded"]);
});

test("unavailable never delivers stale progress or retries automatically", async () => {
  let calls = 0, updates = 0;
  await assert.rejects(watchSimulation(options({ onProgress: () => updates++, fetcher: async () => { calls++; return response('event: unavailable\ndata: {}\n\n' + frame()); } })), /access changed/);
  assert.equal(calls, 1); assert.equal(updates, 0);
});

test("hidden jobs stop immediately, capacity failures use bounded backoff", async () => {
  for (const status of [403, 404, 503]) {
    let calls = 0; const delays = [];
    await assert.rejects(watchSimulation(options({ fetcher: async () => { calls++; return new Response("", { status }); }, delay: async (ms) => { delays.push(ms); } })));
    assert.equal(calls, status === 503 ? 6 : 1);
    if (status === 503) assert.deepEqual(delays, [1000, 2000, 4000, 8000, 16000]);
  }
});

test("abort releases a waiting reader and does not reconnect", async () => {
  const controller = new AbortController();
  let cancelled = false, opened;
  const ready = new Promise((resolve) => { opened = resolve; });
  const running = watchSimulation(options({ signal: controller.signal, fetcher: async () => {
    opened();
    return new Response(new ReadableStream({ cancel() { cancelled = true; } }), { headers: { "Content-Type": "text/event-stream" } });
  } }));
  await ready; controller.abort(); await running;
  assert.equal(cancelled, true);
});

test("rejects another app's snapshot and non-simulated responses", async () => {
  assert.throws(() => parseSimulation({ ...job, simulated: false }));
  await assert.rejects(watchSimulation(options({ fetcher: async () => response(frame({ ...job, applicationId: "another-app" })) })), /Invalid progress/);
});

test("completion frame terminates an unchanged reconnect", async () => {
  let calls = 0;
  await watchSimulation(options({ fetcher: async () => { calls++; return response("event: complete\ndata: {}\n\n"); } }));
  assert.equal(calls, 1);
});

test("abort interrupts reconnect backoff without sending another request", async () => {
  const controller = new AbortController();
  let calls = 0;
  await watchSimulation(options({ signal: controller.signal,
    fetcher: async () => { calls++; return new Response("", { status: 503 }); },
    onState: (state) => { if (state === "reconnecting") controller.abort(); },
  }));
  assert.equal(calls, 1);
});

test("expired token reconnect refreshes authorization", async () => {
  let tokens = 0, calls = 0;
  await watchSimulation(options({ getToken: async () => `test-token-${++tokens}`,
    fetcher: async (_url, init) => {
      assert.equal(init.headers.Authorization, `Bearer test-token-${tokens}`);
      return ++calls === 1 ? new Response("", { status: 401 }) : response(frame({ ...job, status: "cancelled" }));
    },
  }));
  assert.equal(tokens, 2);
});

test("invalid typed fields are rejected and unknown fields are not retained", () => {
  assert.throws(() => parseSimulation({ ...job, failureCode: {} }));
  assert.throws(() => parseSimulation({ ...job, createdAt: "not-a-date" }));
  assert.equal("rawOutput" in parseSimulation({ ...job, rawOutput: "not part of contract" }), false);
});
