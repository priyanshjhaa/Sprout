import test from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync, mkdirSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { FolderProblem, limits, packTar, prepareFolder, skipReason, splitUstarPath } from "../src/lib/folder-archive.ts";

const bytes = (text) => new TextEncoder().encode(text);
const file = (path, content = "x") => {
  const data = typeof content === "string" ? bytes(content) : content;
  return { path, size: data.byteLength, read: async () => data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength) };
};
const huge = (path, size) => ({ path, size, read: async () => new ArrayBuffer(size) });
const paths = (prepared) => prepared.files.map((f) => f.path);
const problem = (fn) => assert.throws(fn, (error) => error instanceof FolderProblem);

test("the dropped folder's own name is not part of the site, and names the app", () => {
  const prepared = prepareFolder([file("my-site/index.html"), file("my-site/css/site.css"), file("my-site/about/index.html")]);
  assert.deepEqual(paths(prepared), ["about/index.html", "css/site.css", "index.html"]);
  assert.equal(prepared.folderName, "my-site");
  assert.equal(prepared.kind, "static");
});

test("loose files dropped together stay at the top, and a zip's double wrapper is looked through", () => {
  assert.deepEqual(paths(prepareFolder([file("index.html"), file("style.css")])), ["index.html", "style.css"]);
  const nested = prepareFolder([file("site/site/index.html"), file("site/site/a.css")]);
  assert.deepEqual(paths(nested), ["a.css", "index.html"]);
  assert.equal(nested.folderName, "site");
  // A folder that really has several top-level entries is left alone.
  assert.deepEqual(paths(prepareFolder([file("p/index.html"), file("p/docs/readme.html"), file("p/docs/more/x.html")])), ["docs/more/x.html", "docs/readme.html", "index.html"]);
});

test("hidden files, secrets and dependency folders are never uploaded, and the person is told", () => {
  const prepared = prepareFolder([
    file("site/index.html"), file("site/.DS_Store"), file("site/.env"), file("site/.git/config"), file("site/__MACOSX/x"),
    file("site/node_modules/left-pad/index.js"), file("site/server.pem"), file("site/id_rsa"), file("site/credentials.json"),
    file("site/Thumbs.db"), file("site/._index.html"), file("site/ok.css"),
  ]);
  assert.deepEqual(paths(prepared), ["index.html", "ok.css"]);
  assert.deepEqual(prepared.skipped, [".DS_Store", "._index.html", ".env", ".git", "Thumbs.db", "__MACOSX", "credentials.json", "id_rsa", "node_modules", "server.pem"]);
  assert.equal(skipReason(".env.production"), "hidden");
  assert.equal(skipReason("keys.PEM"), "private");
  assert.equal(skipReason("node_modules"), "dependencies");
  assert.equal(skipReason("index.html"), null);
  assert.equal(skipReason("environment.html"), null, "only a leading dot hides a file");
  problem(() => prepareFolder([file("site/.env"), file("site/.git/HEAD")]));
  problem(() => prepareFolder([]));
});

test("it says what it found when there is nothing to deploy, and hints at near misses", () => {
  assert.equal(prepareFolder([file("app/package.json"), file("app/server.js")]).kind, "node");
  assert.equal(prepareFolder([file("package.json"), file("index.html")]).kind, "node", "package.json wins, as on the server");
  assert.throws(() => prepareFolder([file("notes/readme.txt"), file("notes/todo.txt")]), /index\.html.*package\.json.*readme\.txt/);
  assert.throws(() => prepareFolder([file("site/Index.html")]), /Rename "Index.html" to exactly "index.html"/);
  assert.throws(() => prepareFolder([file("site/home/index.html"), file("site/other.html")]), /at the top of the folder/);
  // A project whose finished website is in dist/: point at that folder.
  assert.throws(() => prepareFolder([file("proj/README.md"), file("proj/dist/index.html"), file("proj/src/a.js")]), /finished website is in the "dist" folder: drop that folder instead/);
});

test("limits are explained before any upload", () => {
  const many = Array.from({ length: limits.maxFiles + 1 }, (_, i) => file(`s/f${i}.html`));
  many.push(file("s/index.html"));
  assert.throws(() => prepareFolder(many), /up to 256/);
  assert.throws(() => prepareFolder([file("s/index.html"), huge("s/video.mp4", limits.maxFileBytes + 1)]), /video\.mp4/);
  assert.throws(() => prepareFolder([file("s/index.html"), huge("s/a.bin", 2_000_000), huge("s/b.bin", 2_000_000), huge("s/c.bin", 2_000_000), huge("s/d.bin", 2_000_000), huge("s/e.bin", 2_000_000)]), /MiB; Sprout takes up to 8\.0 MiB/);
  assert.doesNotThrow(() => prepareFolder([file("s/index.html"), huge("s/ok.bin", limits.maxFileBytes)]));
  assert.throws(() => prepareFolder([file("s/index.html"), file("s/" + "d/".repeat(17) + "f.html")]), /deep/);
});

test("file names a website can't carry are listed, in words", () => {
  assert.throws(() => prepareFolder([file("s/index.html"), file("s/my photo.png"), file("s/café.html")]), /"my photo\.png", "café\.html".*no spaces/);
  assert.throws(() => prepareFolder([file("s/index.html"), file("s/Index.HTML")]), /twice with different capitalisation/);
  assert.throws(() => prepareFolder([file("s/index.html"), file("s/a:b.html")]), /can't be used/);
  assert.throws(() => prepareFolder([file("s/index.html"), file("s/trailing.")]), /can't be used/);
  assert.throws(() => prepareFolder([file("s/index.html"), file("s/a\\b.html")]), /can't be used/);
  assert.throws(() => prepareFolder([file("s/index.html"), file("s/line\nbreak.html")]), /can't be used/);
  // Node apps are judged by the server's looser rules, which allow a space.
  assert.doesNotThrow(() => prepareFolder([file("a/package.json"), file("a/my file.js")]));
  problem(() => prepareFolder([file("../escape/index.html")]));
});

test("long paths are split into the tar's name and prefix, or refused", () => {
  const long = "assets/" + "a".repeat(40) + "/" + "b".repeat(40) + "/" + "c".repeat(30) + ".txt";
  const parts = splitUstarPath(long);
  assert.ok(parts && parts.prefix && parts.name.length <= 100 && parts.prefix.length <= 155 && `${parts.prefix}/${parts.name}` === long);
  assert.deepEqual(splitUstarPath("index.html"), { prefix: "", name: "index.html" });
  assert.equal(splitUstarPath("x".repeat(101)), null, "a 101-byte file name with no folder cannot be represented");
  assert.equal(splitUstarPath("d/" + "x".repeat(101)), null);
  assert.throws(() => prepareFolder([file("s/index.html"), file("s/" + "x".repeat(110) + ".html")]), /too long|can't be used/);
});

async function packed(files) {
  return new Uint8Array(await (await packTar(files)).arrayBuffer());
}

test("the archive is a well-formed USTAR tar, byte for byte", async () => {
  const content = "<h1>héllo</h1>";
  const data = await packed(prepareFolder([file("s/index.html", content), file("s/css/a.css", "a{}"), file("s/empty.txt", "")]).files);
  assert.equal(data.length % 512, 0);
  assert.ok(data.slice(-1024).every((b) => b === 0), "ends with two empty blocks");
  const decoder = new TextDecoder();
  const text = (block, from, to) => decoder.decode(block.slice(from, to)).replace(/\0.*$/s, "");
  const seen = [];
  for (let offset = 0; offset < data.length - 1024;) {
    const block = data.slice(offset, offset + 512);
    assert.equal(text(block, 257, 263), "ustar");
    assert.equal(decoder.decode(block.slice(263, 265)), "00");
    assert.equal(String.fromCharCode(block[156]), "0", "regular files only");
    const copy = block.slice(); copy.fill(0x20, 148, 156);
    assert.equal(parseInt(text(block, 148, 154), 8), copy.reduce((a, b) => a + b, 0), "checksum");
    const size = parseInt(text(block, 124, 135), 8);
    seen.push([text(block, 0, 100), size, decoder.decode(data.slice(offset + 512, offset + 512 + size))]);
    offset += 512 + Math.ceil(size / 512) * 512;
  }
  assert.deepEqual(seen, [["css/a.css", 3, "a{}"], ["empty.txt", 0, ""], ["index.html", bytes(content).length, content]]);
});

test("the system tar reads it, including long paths", async () => {
  const dir = mkdtempSync(join(tmpdir(), "sprout-pack-"));
  try {
    const long = "assets/" + "a".repeat(40) + "/" + "b".repeat(40) + "/" + "c".repeat(30) + ".txt";
    const archive = join(dir, "site.tar");
    writeFileSync(archive, await packed(prepareFolder([file("s/index.html", "home"), file("s/about/index.html", "about"), file(`s/${long}`, "deep")]).files));
    const listing = spawnSync("tar", ["-tf", archive], { encoding: "utf8" });
    assert.equal(listing.status, 0, listing.stderr);
    assert.deepEqual(listing.stdout.trim().split("\n").sort(), [long, "about/index.html", "index.html"].sort());
    assert.equal(spawnSync("tar", ["-xOf", archive, long], { encoding: "utf8" }).stdout, "deep");
    mkdirSync(join(dir, "out"));
    assert.equal(spawnSync("tar", ["-xf", archive, "-C", join(dir, "out")]).status, 0);
    assert.equal(readFileSync(join(dir, "out", "about", "index.html"), "utf8"), "about");
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("packing is reproducible, and a file that changes mid-way is caught", async () => {
  const files = prepareFolder([file("s/index.html", "same"), file("s/b.css", "b")]).files;
  assert.deepEqual(await packed(files), await packed(files));
  const shrinking = { path: "index.html", size: 10, read: async () => new ArrayBuffer(4) };
  await assert.rejects(packTar([shrinking]), (error) => error instanceof FolderProblem && /changed while/.test(error.message));
  const vanishing = { path: "index.html", size: 1, read: async () => { throw new Error("NotFoundError"); } };
  await assert.rejects(packTar([vanishing]), (error) => error instanceof FolderProblem && /could not be read/.test(error.message));
});

// The Go server must accept exactly what the browser packs. This fixture is the
// packer's output for a fixed folder; backend/internal/staticsite has a test that
// feeds the same bytes through the real archive extractor and static contract.
// After changing the packer, regenerate with: UPDATE_FIXTURE=1 npm test
test("the packer's output matches the fixture the Go server is tested against", async () => {
  const long = "assets/" + "a".repeat(40) + "/" + "b".repeat(40) + "/" + "c".repeat(30) + ".txt";
  const data = await packed(prepareFolder([
    file("demo/index.html", "<h1>Home</h1>"), file("demo/about/index.html", "<h1>About</h1>"), file("demo/css/site.css", "h1{color:green}"),
    file("demo/404.html", "<h1>Missing</h1>"), file(`demo/${long}`, "deep file"), file("demo/.DS_Store", "junk"),
  ]).files);
  const fixture = new URL("../../backend/internal/staticsite/testdata/folder-packed.tar", import.meta.url);
  if (process.env.UPDATE_FIXTURE === "1") {
    mkdirSync(new URL(".", fixture), { recursive: true });
    writeFileSync(fixture, data);
  }
  assert.deepEqual(new Uint8Array(readFileSync(fixture)), data);
});
