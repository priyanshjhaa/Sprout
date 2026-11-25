import test from "node:test";
import assert from "node:assert/strict";
import { describeNodeProblems, inspectNodeApp } from "../src/lib/node-readiness.ts";

const ok = { name: "app", version: "1.0.0", private: true, engines: { node: "24.x" }, scripts: { build: "node build.mjs", start: "node dist/server.mjs" } };
const file = (path, text) => { const data = new TextEncoder().encode(text); return { path, size: data.length, read: async () => data.buffer.slice(0) }; };

test("an app that meets the rules has nothing to report", () => {
  assert.deepEqual(describeNodeProblems(ok, true), []);
});

test("packages are fine now: a real project is told only what it actually breaks", () => {
  // CompIQ-shaped: a real Next.js app with no engines field.
  const compiq = { name: "compiq", version: "0.1.0", private: true, scripts: { build: "next build", start: "next start" },
    dependencies: { next: "16.2.7", react: "19.2.4", zod: "^4" }, devDependencies: { typescript: "^5" } };
  assert.deepEqual(describeNodeProblems(compiq, true), ['package.json needs "engines": { "node": "24.x" }.']);
  assert.deepEqual(describeNodeProblems({ ...ok, dependencies: { express: "^5" } }, true), []);
});

test("a project without a start script builds a website, which is fine", () => {
  assert.deepEqual(describeNodeProblems({ ...ok, scripts: { build: "vite build" }, devDependencies: { vite: "^7" } }, true), []);
});

test("a built dist folder is mentioned only alongside a real problem", () => {
  assert.deepEqual(describeNodeProblems(ok, true, "dist"), []);
  const problems = describeNodeProblems({ ...ok, private: false }, true, "dist");
  assert.equal(problems.length, 2);
  assert.match(problems[1], /drop its "dist" folder on its own/);
});

test("each unmet rule is named in words", () => {
  assert.deepEqual(describeNodeProblems({ ...ok, engines: undefined }, true), ['package.json needs "engines": { "node": "24.x" }.']);
  assert.deepEqual(describeNodeProblems({ ...ok, private: false }, true), ['package.json needs "private": true.']);
  assert.match(describeNodeProblems({ ...ok, scripts: {} }, true)[0], /needs a "build" script/);
  assert.match(describeNodeProblems({ ...ok, scripts: { build: " ", start: "x" } }, true)[0], /needs a "build" script/);
  assert.match(describeNodeProblems({ ...ok, scripts: { build: "x", start: " " } }, true)[0], /"start" script in package\.json is empty/);
  assert.deepEqual(describeNodeProblems({ ...ok, workspaces: ["a/*"] }, true), ["Workspaces (monorepos) aren't supported."]);
  assert.deepEqual(describeNodeProblems(ok, false), ['Add a package-lock.json (run "npm install" once to create it).']);
  assert.deepEqual(describeNodeProblems({ name: "x" }, true).length, 4, "no version, private, engines, scripts");
  for (const junk of [null, undefined, "text", 5, [], ]) assert.match(describeNodeProblems(junk, true)[0], /couldn't be read/);
});

test("it reads package.json from the dropped files, and survives a broken one", async () => {
  const files = (manifest, extra = []) => [file("package.json", manifest), file("package-lock.json", "{}"), ...extra];
  assert.deepEqual(await inspectNodeApp(files(JSON.stringify(ok))), []);
  assert.deepEqual(await inspectNodeApp(files(JSON.stringify({ ...ok, dependencies: { express: "^4" } }), [file("dist/index.html", "x")])), []);
  assert.match((await inspectNodeApp(files("{ not json")))[0], /couldn't be read/);
  assert.match((await inspectNodeApp([file("package.json", JSON.stringify(ok))]))[0], /package-lock\.json/);
  assert.deepEqual(await inspectNodeApp([file("index.html", "x")]), [], "no package.json: nothing to inspect");
  assert.match((await inspectNodeApp([{ path: "package.json", size: 300_000, read: async () => new ArrayBuffer(0) }]))[0], /unusually large/);
});
