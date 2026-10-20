// Tells someone, in plain words and before they upload, whether Sprout can run
// the Node.js app in the folder they dropped. It mirrors the server's rules in
// backend/internal/nodeapp, but only to explain: the server stays the authority,
// so these are warnings, never a lock.

import type { SourceFile } from "./folder-archive.ts";

const MAX_MANIFEST_BYTES = 256 * 1024;
// Where a build tool puts the finished website.
const builtOutputFolders = ["dist", "build", "out"];

const quote = (names: string[], max = 4) =>
  names.slice(0, max).join(", ") + (names.length > max ? ` and ${names.length - max} more` : "");

function packageNames(manifest: Record<string, unknown>): string[] {
  const names = new Set<string>();
  for (const field of ["dependencies", "devDependencies", "optionalDependencies"]) {
    const group = manifest[field];
    if (group && typeof group === "object") Object.keys(group).forEach((name) => names.add(name));
  }
  return [...names];
}

/**
 * What would stop this Node.js app from running on Sprout today, as sentences.
 * Empty means nothing obvious. `builtOutput` is the name of a folder that already
 * holds a finished website (like "dist"), if the project has one.
 */
export function describeNodeProblems(manifest: unknown, hasLockfile: boolean, builtOutput: string | null = null): string[] {
  if (!manifest || typeof manifest !== "object" || Array.isArray(manifest)) {
    return ["package.json couldn't be read. Check that it is valid JSON."];
  }
  const m = manifest as Record<string, unknown>;

  // Packages are the blocker no edit can fix today, so they are said first and alone.
  const packages = packageNames(m);
  if (packages.length > 0) {
    const problems = [`Sprout can't install packages yet, and this app uses ${packages.length} (${quote(packages)}). Plain websites, and Node.js apps that use no packages, work today.`];
    if (builtOutput) {
      problems.push(`If this is a website you built, drop its "${builtOutput}" folder instead. That is a plain website Sprout can run now.`);
    }
    return problems;
  }

  const problems: string[] = [];
  const engines = m.engines && typeof m.engines === "object" ? (m.engines as Record<string, unknown>) : {};
  const scripts = m.scripts && typeof m.scripts === "object" ? (m.scripts as Record<string, unknown>) : {};
  if (typeof m.name !== "string" || !m.name || typeof m.version !== "string" || !m.version) problems.push('package.json needs a "name" and a "version".');
  if (m.private !== true) problems.push('package.json needs "private": true.');
  if (engines.node !== "24.x") problems.push('package.json needs "engines": { "node": "24.x" }.');
  const missing = ["build", "start"].filter((name) => typeof scripts[name] !== "string" || !(scripts[name] as string).trim());
  if (missing.length > 0) problems.push(`package.json needs a ${missing.map((name) => `"${name}"`).join(" and a ")} script.`);
  if (m.workspaces) problems.push("Workspaces (monorepos) aren't supported.");
  if (!hasLockfile) problems.push('Add a package-lock.json (run "npm install" once to create it).');
  if (builtOutput) problems.push(`If this is a website you built, drop its "${builtOutput}" folder instead.`);
  return problems;
}

/** Reads package.json from a prepared folder and describes any problems. */
export async function inspectNodeApp(files: SourceFile[]): Promise<string[]> {
  const manifestFile = files.find((file) => file.path === "package.json");
  if (!manifestFile) return [];
  const hasLockfile = files.some((file) => file.path === "package-lock.json");
  const builtOutput = builtOutputFolders.find((dir) => files.some((file) => file.path === `${dir}/index.html`)) ?? null;
  if (manifestFile.size > MAX_MANIFEST_BYTES) return ["package.json is unusually large and can't be used."];
  let manifest: unknown;
  try {
    manifest = JSON.parse(new TextDecoder().decode(await manifestFile.read()));
  } catch {
    manifest = null;
  }
  return describeNodeProblems(manifest, hasLockfile, builtOutput);
}
