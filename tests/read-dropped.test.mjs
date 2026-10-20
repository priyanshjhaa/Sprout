import test from "node:test";
import assert from "node:assert/strict";
import { FolderProblem } from "../src/lib/folder-archive.ts";
import { readEntriesForTest, readPicked } from "../src/lib/read-dropped.ts";

// Fakes shaped like the browser's FileSystem entries.
const fileEntry = (name, text = "x", counter = { reads: 0 }) => ({
  isFile: true, isDirectory: false, name,
  file: (ok) => { counter.reads++; ok(new File([text], name)); },
});
const dirEntry = (name, children, counter = { readers: 0 }, batch = 100) => ({
  isFile: false, isDirectory: true, name,
  createReader() {
    counter.readers++;
    let index = 0;
    return { readEntries(ok) { const slice = children.slice(index, index + batch); index += batch; ok(slice); } };
  },
});

test("a dropped folder is read recursively, in batches, and skipped folders are never entered", async () => {
  const modules = { readers: 0 };
  const git = { readers: 0 };
  const many = Array.from({ length: 250 }, (_, i) => fileEntry(`p${i}.html`));
  const site = dirEntry("site", [
    fileEntry("index.html"), dirEntry("pages", many), dirEntry("node_modules", [fileEntry("x.js")], modules),
    dirEntry(".git", [fileEntry("HEAD")], git), fileEntry(".DS_Store"),
  ]);
  const result = await readEntriesForTest([site]);
  assert.equal(result.kind, "folder");
  assert.equal(result.files.length, 251, "all three read batches of 'pages' plus index.html");
  assert.ok(result.files.some((f) => f.path === "site/pages/p249.html"));
  assert.ok(result.files.every((f) => f.path.startsWith("site/")));
  assert.deepEqual(result.skipped.sort(), [".DS_Store", ".git", "node_modules"]);
  assert.equal(modules.readers + git.readers, 0, "skipped folders are not even listed");
  const first = result.files.find((f) => f.path === "site/index.html");
  assert.equal(new TextDecoder().decode(await first.read()), "x");
});

test("several loose files and folders dropped together keep their own names", async () => {
  const result = await readEntriesForTest([fileEntry("index.html"), dirEntry("css", [fileEntry("a.css")])]);
  assert.deepEqual(result.files.map((f) => f.path).sort(), ["css/a.css", "index.html"]);
});

test("a single .tar is passed through; a zip or stray file gets a helpful message", async () => {
  const tar = new File(["x"], "site.tar");
  const archive = await readEntriesForTest([fileEntry("site.tar")], [tar]);
  assert.equal(archive.kind, "archive");
  assert.equal(archive.file, tar);
  await assert.rejects(readEntriesForTest([fileEntry("site.zip")], [new File(["x"], "site.zip")]), (e) => e instanceof FolderProblem && /Unzip it/.test(e.message));
  await assert.rejects(readEntriesForTest([fileEntry("photo.png")], [new File(["x"], "photo.png")]), (e) => e instanceof FolderProblem && /Drop the folder/.test(e.message));
  await assert.rejects(readEntriesForTest([]), (e) => e instanceof FolderProblem);
});

test("a huge folder is refused quickly instead of freezing the page", async () => {
  const big = dirEntry("project", Array.from({ length: 4000 }, (_, i) => fileEntry(`f${i}.js`)));
  await assert.rejects(readEntriesForTest([big]), (e) => e instanceof FolderProblem && /too many files/.test(e.message));
});

test("a picked folder is filtered by name, since the picker lists every file", () => {
  const picked = (path) => Object.assign(new File(["x"], path.split("/").pop()), { webkitRelativePath: path });
  const result = readPicked([picked("site/index.html"), picked("site/node_modules/a/b.js"), picked("site/.env"), picked("site/css/a.css")]);
  assert.equal(result.kind, "folder");
  assert.deepEqual(result.files.map((f) => f.path), ["site/index.html", "site/css/a.css"]);
  assert.deepEqual(result.skipped.sort(), [".env", "node_modules"]);
  assert.equal(readPicked([new File(["x"], "app.tar")]).kind, "archive");
  assert.throws(() => readPicked([new File(["x"], "site.zip")]), /Unzip it/);
});
