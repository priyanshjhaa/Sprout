import test from "node:test";
import assert from "node:assert/strict";
import { describeNodeProblems, inspectNodeApp } from "../src/lib/node-readiness.ts";

const ok = { name: "app", version: "1.0.0", private: true, engines: { node: "24.x" }, scripts: { build: "node build.mjs", start: "node dist/server.mjs" } };
const file = (path, text) => { const data = new TextEncoder().encode(text); return { path, size: data.length, read: async () => data.buffer.slice(0) }; };

test("an app that meets the rules has nothing to report", () => {
  assert.deepEqual(describeNodeProblems(ok, true), []);
});

test("packages are the headline, said alone, with the project's own words", () => {
  // CompIQ-shaped: a real Next.js app with no engines field.
  const compiq = { name: "compiq", version: "0.1.0", private: true, scripts: { build: "next build", start: "next start" },
    dependencies: { next: "16.2.7", react: "19.2.4", zod: "^4", prisma: "^6", "@prisma/client": "^6" }, devDependencies: { typescript: "^5" } };
  const problems = describeNodeProblems(compiq, true);
  assert.equal(problems.length, 1, "the fixable-but-pointless engines rule is not mentioned");
  assert.match(problems[0], /can't install packages yet, and this app uses 6 \(next, react, zod, prisma and 2 more\)/);
  assert.match(problems[0], /Plain websites, and Node\.js apps that use no packages, work today/);
});

test("a project with a finished site in dist is pointed at it", () => {
  const withPackages = describeNodeProblems({ ...ok, dependencies: { vite: "^5" } }, true, "dist");
  assert.equal(withPackages.length, 2);
  assert.match(withPackages[1], /drop its "dist" folder instead/);
  assert.equal(describeNodeProblems({ ...ok, dependencies: { vite: "^5" } }, true, null).length, 1);
});

test("without packages, each unmet rule is named in words", () => {
  assert.deepEqual(describeNodeProblems({ ...ok, engines: undefined }, true), ['package.json needs "engines": { "node": "24.x" }.']);
  assert.deepEqual(describeNodeProblems({ ...ok, private: false }, true), ['package.json needs "private": true.']);
  assert.deepEqual(describeNodeProblems({ ...ok, scripts: { build: "x" } }, true), ['package.json needs a "start" script.']);
  assert.deepEqual(describeNodeProblems({ ...ok, scripts: {} }, true), ['package.json needs a "build" and a "start" script.']);
  assert.deepEqual(describeNodeProblems({ ...ok, scripts: { build: " ", start: "x" } }, true), ['package.json needs a "build" script.']);
  assert.deepEqual(describeNodeProblems({ ...ok, workspaces: ["a/*"] }, true), ["Workspaces (monorepos) aren't supported."]);
  assert.deepEqual(describeNodeProblems(ok, false), ['Add a package-lock.json (run "npm install" once to create it).']);
  assert.deepEqual(describeNodeProblems({ name: "x" }, true).length, 4, "no version, private, engines, scripts");
  for (const junk of [null, undefined, "text", 5, [], ]) assert.match(describeNodeProblems(junk, true)[0], /couldn't be read/);
});

test("it reads package.json from the dropped files, and survives a broken one", async () => {
  const files = (manifest, extra = []) => [file("package.json", manifest), file("package-lock.json", "{}"), ...extra];
  assert.deepEqual(await inspectNodeApp(files(JSON.stringify(ok))), []);
  const withDeps = await inspectNodeApp(files(JSON.stringify({ ...ok, dependencies: { express: "^4" } }), [file("dist/index.html", "x")]));
  assert.equal(withDeps.length, 2);
  assert.match(withDeps[0], /uses 1 \(express\)/);
  assert.match((await inspectNodeApp(files("{ not json")))[0], /couldn't be read/);
  assert.match((await inspectNodeApp([file("package.json", JSON.stringify(ok))]))[0], /package-lock\.json/);
  assert.deepEqual(await inspectNodeApp([file("index.html", "x")]), [], "no package.json: nothing to inspect");
  assert.match((await inspectNodeApp([{ path: "package.json", size: 300_000, read: async () => new ArrayBuffer(0) }]))[0], /unusually large/);
});
