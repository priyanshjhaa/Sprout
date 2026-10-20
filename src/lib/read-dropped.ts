// Reads what the browser hands over when someone drops or picks something, and
// says what it is: a ready-made .tar archive, or a folder of files.

import { FolderProblem, skipReason, type SourceFile } from "./folder-archive.ts";

export type Dropped =
  | { kind: "archive"; file: File }
  | { kind: "folder"; files: SourceFile[]; skipped: string[] };

// Enough for any real site, small enough that a dropped developer project
// (thousands of files) is refused quickly instead of freezing the tab.
const MAX_VISITED = 3000;

// The slice of the (non-standard but universal) FileSystem entry API we use,
// declared narrowly so it can be exercised without a browser.
export interface EntryLike {
  isFile: boolean;
  isDirectory: boolean;
  name: string;
  file?(success: (file: File) => void, failure: (error: unknown) => void): void;
  createReader?(): { readEntries(success: (entries: EntryLike[]) => void, failure: (error: unknown) => void): void };
}

const isTar = (file: File) => /\.tar$/i.test(file.name);

function describeSingleFile(file: File): never {
  if (/\.zip$/i.test(file.name)) {
    throw new FolderProblem("That's a zip file. Unzip it and drop the folder inside instead. Zip files aren't supported yet.");
  }
  throw new FolderProblem(`Drop the folder that contains your site (with an index.html), not a single ${file.name.includes(".") ? "file" : "item"}. A .tar archive also works.`);
}

function fileOf(entry: EntryLike): Promise<File> {
  return new Promise((resolve, reject) => entry.file!(resolve, reject));
}

async function readAllEntries(directory: EntryLike): Promise<EntryLike[]> {
  const reader = directory.createReader!();
  const all: EntryLike[] = [];
  // readEntries returns at most ~100 entries per call; keep asking until it is empty.
  for (;;) {
    const batch = await new Promise<EntryLike[]>((resolve, reject) => reader.readEntries(resolve, reject));
    if (batch.length === 0) return all;
    all.push(...batch);
  }
}

async function walk(entry: EntryLike, prefix: string, found: SourceFile[], skipped: Set<string>, visited: { count: number }): Promise<void> {
  if (++visited.count > MAX_VISITED) {
    throw new FolderProblem("That folder has too many files to be a site. Drop just the folder that contains your index.html.");
  }
  const path = prefix + entry.name;
  // Never walk into folders that are never uploaded: it keeps big projects fast.
  // (The wrapper folder itself is the first segment and is judged by the caller.)
  if (prefix !== "" && skipReason(entry.name)) {
    skipped.add(entry.name);
    return;
  }
  if (entry.isFile) {
    const file = await fileOf(entry);
    found.push({ path, size: file.size, read: () => file.arrayBuffer() });
    return;
  }
  if (entry.isDirectory) {
    for (const child of await readAllEntries(entry)) await walk(child, `${path}/`, found, skipped, visited);
  }
}

/**
 * Reads a drop. Call it directly from the drop handler, before awaiting
 * anything: the browser invalidates the dropped items once the handler yields.
 */
export function readDropped(items: DataTransferItemList): Promise<Dropped> {
  // Take everything we need from the event synchronously (see above).
  const taken = Array.from(items)
    .filter((item) => item.kind === "file")
    .map((item) => ({ entry: (item.webkitGetAsEntry?.() ?? null) as EntryLike | null, file: item.getAsFile() }));
  return readTaken(taken);
}

async function readTaken(taken: Array<{ entry: EntryLike | null; file: File | null }>): Promise<Dropped> {
  if (taken.length === 0) throw new FolderProblem("Nothing was dropped. Drop the folder that contains your site.");
  if (taken.length === 1) {
    const only = taken[0];
    const isDirectory = only.entry?.isDirectory ?? false;
    if (!isDirectory && only.file) {
      if (isTar(only.file)) return { kind: "archive", file: only.file };
      describeSingleFile(only.file);
    }
  }
  const found: SourceFile[] = [];
  const skipped = new Set<string>();
  const visited = { count: 0 };
  for (const { entry, file } of taken) {
    if (entry) await walk(entry, "", found, skipped, visited);
    else if (file) found.push({ path: file.name, size: file.size, read: () => file.arrayBuffer() });
  }
  return { kind: "folder", files: found, skipped: [...skipped] };
}

/** Reads the result of an `<input type=file webkitdirectory>` (or a multi-file input). */
export function readPicked(list: FileList): Dropped {
  const files = Array.from(list);
  if (files.length === 1 && !files[0].webkitRelativePath) {
    if (isTar(files[0])) return { kind: "archive", file: files[0] };
    describeSingleFile(files[0]);
  }
  const skipped = new Set<string>();
  const found: SourceFile[] = [];
  for (const file of files) {
    const path = file.webkitRelativePath || file.name;
    const segments = path.split("/");
    // The picker lists every file, even inside node_modules: filter by name, not by walking.
    const blocked = segments.slice(1).find((segment) => skipReason(segment));
    if (blocked) {
      skipped.add(blocked);
      continue;
    }
    found.push({ path, size: file.size, read: () => file.arrayBuffer() });
  }
  if (found.length > MAX_VISITED) {
    throw new FolderProblem("That folder has too many files to be a site. Choose just the folder that contains your index.html.");
  }
  return { kind: "folder", files: found, skipped: [...skipped] };
}

// Exposed for tests: the same logic without needing a DataTransferItemList.
export const readEntriesForTest = (entries: EntryLike[], files: Array<File | null> = []) =>
  readTaken(entries.map((entry, index) => ({ entry, file: files[index] ?? null })));
