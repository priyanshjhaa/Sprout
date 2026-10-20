// What a person gave Sprout to deploy, in the words the screens use. Shared by
// the "drop anywhere" dialog and the upload areas so both behave the same.

import { FolderProblem, packTar, prepareFolder, type PreparedFolder } from "./folder-archive.ts";
import type { Dropped } from "./read-dropped.ts";

export const MAX_ARCHIVE_BYTES = 16 * 1024 * 1024;

export type Chosen =
  | { kind: "archive"; file: File }
  | { kind: "folder"; folder: PreparedFolder };

/** Checks what was dropped or picked. Throws a FolderProblem the person can act on. */
export function chooseSource(dropped: Dropped): Chosen {
  if (dropped.kind === "archive") {
    if (dropped.file.size > MAX_ARCHIVE_BYTES) throw new FolderProblem("That .tar file is larger than 16 MiB.");
    return { kind: "archive", file: dropped.file };
  }
  const folder = prepareFolder(dropped.files);
  // Folders skipped while reading (never walked) still count as left out.
  return { kind: "folder", folder: { ...folder, skipped: [...new Set([...folder.skipped, ...dropped.skipped])].sort() } };
}

/** The bytes to upload: the .tar as given, or the folder packed in the browser. */
export function sourceBlob(chosen: Chosen): Promise<Blob> {
  return chosen.kind === "archive" ? Promise.resolve(chosen.file) : packTar(chosen.folder.files);
}

/** A starting name for the app, from the folder or archive name. */
export function suggestedName(chosen: Chosen): string {
  const raw = chosen.kind === "archive" ? chosen.file.name.replace(/\.tar$/i, "") : chosen.folder.folderName ?? "";
  return raw.replace(/[-_]+/g, " ").trim() || "My site";
}

const size = (bytes: number) => bytes >= 1024 * 1024 ? `${(bytes / (1024 * 1024)).toFixed(1)} MiB` : `${Math.max(1, Math.round(bytes / 1024))} KiB`;

/** One line saying what Sprout understood. */
export function summarise(chosen: Chosen): string {
  if (chosen.kind === "archive") return `.tar archive · ${size(chosen.file.size)}`;
  const { folder } = chosen;
  const what = folder.kind === "static" ? "Website" : "Node.js app";
  return `${what} · ${folder.files.length} ${folder.files.length === 1 ? "file" : "files"} · ${size(folder.totalBytes)}`;
}

/** A short, honest note about what was not uploaded, or null. */
export function skippedNote(chosen: Chosen): string | null {
  if (chosen.kind !== "folder" || chosen.folder.skipped.length === 0) return null;
  const shown = chosen.folder.skipped.slice(0, 5).join(", ");
  const more = chosen.folder.skipped.length > 5 ? ` and ${chosen.folder.skipped.length - 5} more` : "";
  return `Left out, never uploaded: ${shown}${more}. These are hidden, private or dependency files.`;
}

/** Turns anything thrown while preparing into a message for the person. */
export const problemMessage = (error: unknown): string =>
  error instanceof FolderProblem ? error.message : error instanceof Error ? error.message : "Sprout could not read that.";
