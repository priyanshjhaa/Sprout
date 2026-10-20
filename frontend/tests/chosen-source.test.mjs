import test from "node:test";
import assert from "node:assert/strict";
import { FolderProblem } from "../src/lib/folder-archive.ts";
import { MAX_ARCHIVE_BYTES, chooseSource, problemMessage, skippedNote, sourceBlob, suggestedName, summarise } from "../src/lib/chosen-source.ts";

const file = (path, text = "x") => { const data = new TextEncoder().encode(text); return { path, size: data.length, read: async () => data.buffer.slice(0) }; };

test("a folder is summarised in plain words, and named after itself", () => {
  const chosen = chooseSource({ kind: "folder", files: [file("my-cool_site/index.html", "a".repeat(2048)), file("my-cool_site/b.css")], skipped: [] });
  assert.equal(summarise(chosen), "Website · 2 files · 2 KiB");
  assert.equal(suggestedName(chosen), "my cool site");
  const single = chooseSource({ kind: "folder", files: [file("x/index.html")], skipped: [] });
  assert.equal(summarise(single), "Website · 1 file · 1 KiB");
  assert.equal(summarise(chooseSource({ kind: "folder", files: [file("a/package.json"), file("a/server.js")], skipped: [] })), "Node.js app · 2 files · 1 KiB");
});

test("loose files get a friendly default name, and a .tar is named after the file", () => {
  assert.equal(suggestedName(chooseSource({ kind: "folder", files: [file("index.html")], skipped: [] })), "My site");
  assert.equal(suggestedName(chooseSource({ kind: "archive", file: new File(["x"], "Cool-App.TAR") })), "Cool App");
  assert.equal(summarise(chooseSource({ kind: "archive", file: new File(["x"], "a.tar") })), ".tar archive · 1 KiB");
});

test("what was left out is reported once, from both the reader and the preparer", () => {
  const chosen = chooseSource({ kind: "folder", files: [file("s/index.html"), file("s/.env"), file("s/node_modules/a.js")], skipped: ["node_modules", ".git"] });
  assert.deepEqual(chosen.folder.skipped, [".env", ".git", "node_modules"]);
  assert.match(skippedNote(chosen), /^Left out, never uploaded: \.env, \.git, node_modules\./);
  assert.equal(skippedNote(chooseSource({ kind: "folder", files: [file("index.html")], skipped: [] })), null);
  assert.equal(skippedNote(chooseSource({ kind: "archive", file: new File(["x"], "a.tar") })), null);
  const many = chooseSource({ kind: "folder", files: [file("index.html")], skipped: ["a", "b", "c", "d", "e", "f", "g"] });
  assert.match(skippedNote(many), /a, b, c, d, e and 2 more/);
});

test("an oversized archive and an unusable folder are refused in plain words", () => {
  const big = { name: "big.tar", size: MAX_ARCHIVE_BYTES + 1 };
  assert.throws(() => chooseSource({ kind: "archive", file: big }), /larger than 16 MiB/);
  assert.throws(() => chooseSource({ kind: "folder", files: [file("notes/a.txt")], skipped: [] }), FolderProblem);
  assert.equal(problemMessage(new FolderProblem("Plain words.")), "Plain words.");
  assert.equal(problemMessage("weird"), "Sprout could not read that.");
});

test("the upload is the .tar as given, or the folder packed", async () => {
  const tar = new File(["raw"], "a.tar");
  assert.equal(await sourceBlob({ kind: "archive", file: tar }), tar);
  const packed = await sourceBlob(chooseSource({ kind: "folder", files: [file("s/index.html", "hi")], skipped: [] }));
  assert.equal(packed.type, "application/x-tar");
  assert.equal(packed.size, 512 + 512 + 1024, "one header block, one data block, two end blocks");
});
