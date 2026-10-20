// Turns a folder the user dropped or picked into the plain tar archive the
// deploy API accepts, so people never have to know what a .tar is.
//
// The server stays the authority: it re-checks every rule below and rejects
// anything unsafe. These checks exist to explain problems in plain words before
// an upload, and to leave out files nobody means to publish (hidden files,
// secrets, dependency folders). Keep the limits in step with
// backend/internal/source/archive.go.

export const limits = {
  maxFiles: 256,
  maxFileBytes: 2 * 1024 * 1024,
  maxTotalBytes: 8 * 1024 * 1024,
  maxDepth: 16,
  maxPathBytes: 240,
} as const;

export interface SourceFile {
  /** Path relative to the dropped folder, with "/" separators. */
  path: string;
  size: number;
  read(): Promise<ArrayBuffer>;
}

/** A problem the person can act on. The message is written for them. */
export class FolderProblem extends Error {
  constructor(message: string) {
    super(message);
    this.name = "FolderProblem";
  }
}

export interface PreparedFolder {
  /** Files to upload, sorted, with paths relative to the app's root. */
  files: SourceFile[];
  /** A website (index.html) or a Node.js app (package.json). */
  kind: "static" | "node";
  /** The dropped folder's own name, a good starting point for the app's name. */
  folderName: string | null;
  totalBytes: number;
  /** What was left out, for a short "we skipped…" note. */
  skipped: string[];
}

const encoder = new TextEncoder();

const secretNames = new Set(["credentials.json", "id_rsa", "id_ed25519", "id_ecdsa"]);
const junkNames = new Set(["__macosx", "thumbs.db", "desktop.ini", "node_modules"]);
const secretSuffixes = [".pem", ".key", ".p12", ".pfx"];

/**
 * Why a path segment is left out of a deployment, or null if it is kept.
 * Hidden files (a leading dot) cover .git, .env, .ssh, .DS_Store and friends in
 * one rule; the rest are secrets and folders Sprout never uploads.
 */
export function skipReason(segment: string): "hidden" | "private" | "dependencies" | "junk" | null {
  const lower = segment.toLowerCase();
  if (lower.startsWith("._")) return "junk"; // macOS resource forks
  if (lower.startsWith(".")) return "hidden";
  if (lower === "node_modules") return "dependencies";
  if (junkNames.has(lower)) return "junk";
  if (secretNames.has(lower) || secretSuffixes.some((suffix) => lower.endsWith(suffix))) return "private";
  return null;
}

// Names a website can safely be served under: mirrors backend/internal/staticsite.
const webSegment = /^[A-Za-z0-9_@+-][A-Za-z0-9._@+-]*$/;

function normalise(path: string): string[] {
  const segments = path.split("/").filter((segment) => segment !== "" && segment !== ".");
  if (segments.length === 0 || segments.includes("..")) {
    throw new FolderProblem("That folder contains a path Sprout can't use.");
  }
  return segments;
}

function commonTop(paths: string[][]): string | null {
  if (paths.length === 0 || paths.some((segments) => segments.length < 2)) return null;
  const top = paths[0][0];
  return paths.every((segments) => segments[0] === top) ? top : null;
}

const hasRootMarker = (paths: string[][]) =>
  paths.some((segments) => segments.length === 1 && (segments[0] === "index.html" || segments[0] === "package.json"));

const listNames = (names: string[], max = 4) =>
  names.slice(0, max).map((name) => `"${name}"`).join(", ") + (names.length > max ? ` and ${names.length - max} more` : "");

const formatBytes = (bytes: number) => bytes >= 1024 * 1024 ? `${(bytes / (1024 * 1024)).toFixed(1)} MiB` : `${Math.max(1, Math.round(bytes / 1024))} KiB`;

/**
 * Decides what a dropped folder is and whether it can be deployed. Throws a
 * FolderProblem, in plain language, when it can't.
 */
export function prepareFolder(picked: SourceFile[]): PreparedFolder {
  if (picked.length === 0) throw new FolderProblem("That folder is empty. Drop the folder that contains your site's files.");

  let entries = picked.map((file) => ({ file, segments: normalise(file.path) }));
  let folderName: string | null = null;
  const strip = (): boolean => {
    const top = commonTop(entries.map((entry) => entry.segments));
    if (top === null) return false;
    folderName ??= top;
    entries = entries.map((entry) => ({ ...entry, segments: entry.segments.slice(1) }));
    return true;
  };

  // The folder's own name is not part of the site: "my-site/index.html" is "index.html".
  strip();

  const skipped = new Set<string>();
  entries = entries.filter((entry) => {
    const blocked = entry.segments.find((segment) => skipReason(segment));
    if (blocked) skipped.add(blocked);
    return !blocked;
  });
  if (entries.length === 0) {
    throw new FolderProblem(`Everything in that folder is hidden or private (${listNames([...skipped])}), so there is nothing to deploy.`);
  }

  // Zipped downloads often unpack to a folder inside a folder: look one or two levels in.
  for (let level = 0; level < 3 && !hasRootMarker(entries.map((entry) => entry.segments)); level++) {
    if (!strip()) break;
  }

  const roots = entries.filter((entry) => entry.segments.length === 1).map((entry) => entry.segments[0]);
  const kind = roots.includes("package.json") ? "node" : roots.includes("index.html") ? "static" : null;
  if (kind === null) {
    const near = roots.find((name) => /^index\.html?$/i.test(name));
    const found = [...new Set(entries.map((entry) => entry.segments[0]))];
    throw new FolderProblem(
      (near ? `Rename "${near}" to exactly "index.html". ` : "")
      + `Sprout needs an index.html (for a website) or a package.json (for a Node.js app) at the top of the folder. I found: ${listNames(found)}.`,
    );
  }

  if (entries.length > limits.maxFiles) {
    throw new FolderProblem(`That folder has ${entries.length} files; Sprout takes up to ${limits.maxFiles}. Remove what the site doesn't need.`);
  }
  const tooBig = entries.filter((entry) => entry.file.size > limits.maxFileBytes).map((entry) => entry.segments.join("/"));
  if (tooBig.length > 0) {
    throw new FolderProblem(`Files over ${formatBytes(limits.maxFileBytes)} can't be deployed: ${listNames(tooBig)}. Compress or remove them.`);
  }
  const totalBytes = entries.reduce((sum, entry) => sum + entry.file.size, 0);
  if (totalBytes > limits.maxTotalBytes) {
    throw new FolderProblem(`That folder is ${formatBytes(totalBytes)}; Sprout takes up to ${formatBytes(limits.maxTotalBytes)}.`);
  }

  const seen = new Set<string>();
  const badNames: string[] = [];
  for (const { segments } of entries) {
    const path = segments.join("/");
    const bytes = encoder.encode(path).length;
    if (segments.length > limits.maxDepth) throw new FolderProblem(`"${path}" is nested more than ${limits.maxDepth} folders deep.`);
    if (bytes > limits.maxPathBytes || splitUstarPath(path) === null) throw new FolderProblem(`"${path}" has a path that is too long.`);
    if (seen.has(path.toLowerCase())) throw new FolderProblem(`"${path}" appears twice with different capitalisation. Rename one.`);
    seen.add(path.toLowerCase());
    // The rules every deployment shares, then the stricter ones websites need.
    const unsafe = segments.some((segment) => /[\\:\u0000-\u001f\u007f]/.test(segment) || segment !== segment.trim() || segment.endsWith("."));
    const notWebSafe = kind === "static" && segments.some((segment) => segment.length > 128 || !webSegment.test(segment));
    if (unsafe || notWebSafe) badNames.push(path);
  }
  if (badNames.length > 0) {
    throw new FolderProblem(
      `These file names can't be used: ${listNames(badNames)}. Use only letters, numbers, dots, dashes and underscores (no spaces), then drop the folder again.`,
    );
  }

  const files = entries
    .map((entry) => ({ file: entry.file, path: entry.segments.join("/") }))
    .sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0))
    .map(({ file, path }) => ({ path, size: file.size, read: () => file.read() }));
  return { files, kind, folderName, totalBytes, skipped: [...skipped].sort() };
}

/**
 * USTAR stores a path in a 100-byte name plus an optional 155-byte prefix,
 * split at a "/". Returns null when a path can't be represented.
 */
export function splitUstarPath(path: string): { prefix: string; name: string } | null {
  const bytes = encoder.encode(path);
  if (bytes.length <= 100) return { prefix: "", name: path };
  for (let slash = path.lastIndexOf("/"); slash > 0; slash = path.lastIndexOf("/", slash - 1)) {
    const prefix = path.slice(0, slash);
    const name = path.slice(slash + 1);
    if (encoder.encode(prefix).length <= 155 && encoder.encode(name).length <= 100 && name !== "") return { prefix, name };
  }
  return null;
}

function writeOctal(block: Uint8Array, offset: number, length: number, value: number) {
  const digits = value.toString(8).padStart(length - 1, "0");
  block.set(encoder.encode(digits), offset); // the final byte stays NUL
}

function header(path: string, size: number): Uint8Array<ArrayBuffer> {
  const parts = splitUstarPath(path);
  if (!parts) throw new FolderProblem(`"${path}" has a path that is too long.`);
  const block = new Uint8Array(512);
  block.set(encoder.encode(parts.name), 0);
  block.set(encoder.encode("0000644"), 100); // mode (Sprout normalises it anyway)
  block.set(encoder.encode("0000000"), 108); // uid
  block.set(encoder.encode("0000000"), 116); // gid
  writeOctal(block, 124, 12, size);
  block.set(encoder.encode("00000000000"), 136); // mtime: fixed, so packing is reproducible
  block.fill(0x20, 148, 156); // the checksum is computed with this field as spaces
  block[156] = 0x30; // '0': a regular file
  block.set(encoder.encode("ustar"), 257); // magic "ustar\0"
  block.set(encoder.encode("00"), 263); // version
  block.set(encoder.encode(parts.prefix), 345);
  const sum = block.reduce((total, byte) => total + byte, 0);
  block.set(encoder.encode(sum.toString(8).padStart(6, "0")), 148); // then NUL and a space
  block[154] = 0;
  block[155] = 0x20;
  return block;
}

/**
 * Packs prepared files into a plain USTAR tar, the one format the server
 * accepts: regular files only (directories are implied by paths), no extended
 * headers. Reading each file again here also catches a file that changed or
 * vanished between the drop and the upload.
 */
export async function packTar(files: SourceFile[]): Promise<Blob> {
  const parts: BlobPart[] = [];
  for (const file of files) {
    let data: ArrayBuffer;
    try {
      data = await file.read();
    } catch {
      throw new FolderProblem(`"${file.path}" could not be read. If it changed or moved, drop the folder again.`);
    }
    if (data.byteLength !== file.size) throw new FolderProblem(`"${file.path}" changed while it was being read. Drop the folder again.`);
    parts.push(header(file.path, data.byteLength), data, new Uint8Array((512 - (data.byteLength % 512)) % 512));
  }
  parts.push(new Uint8Array(1024)); // end of archive: two empty blocks
  return new Blob(parts, { type: "application/x-tar" });
}
