// Tells someone, in plain words and before they upload, whether Sprout can run
// the Python app in the folder they dropped. It mirrors the server's rules in
// backend/internal/pythonapp, but only to explain: the server stays the
// authority, so these are warnings, never a lock.

import type { SourceFile } from "./folder-archive.ts";

const MAX_FILE_BYTES = 256 * 1024;
const compileCommand = "uv pip compile requirements.in --generate-hashes -o requirements.txt";

/**
 * What would stop this Python app from running on Sprout today, as sentences.
 * `requirements` and `procfile` are the files' text, or null when missing.
 */
export function describePythonProblems(requirements: string | null, procfile: string | null): string[] {
  const problems: string[] = [];
  if (requirements !== null) {
    // Join "\" continuations and drop comments, as pip does.
    const lines = requirements.replace(/\r\n/g, "\n").replace(/\\\n/g, " ").split("\n")
      .map((line) => line.replace(/(^|\s)#.*$/, "").trim())
      .filter((line) => line !== "");
    if (lines.some((line) => line.startsWith("-"))) {
      problems.push("requirements.txt has options such as -r, -e or --index-url. Sprout installs only the packages listed, from PyPI.");
    }
    const packages = lines.filter((line) => !line.startsWith("-"));
    if (packages.some((line) => !/^[A-Za-z0-9][A-Za-z0-9._-]*(\[[^\]]*\])?\s*==\s*[^\s;*]+/.test(line))) {
      problems.push(`Every package in requirements.txt must be pinned with ==. Create the file with "${compileCommand}".`);
    } else if (packages.some((line) => !/--hash=sha256:[0-9a-f]{64}/.test(line))) {
      problems.push(`requirements.txt needs the hashes of every package. Create it with "${compileCommand}".`);
    }
  }
  const web = (procfile ?? "").split(/\r?\n/).map((line) => line.trim()).filter((line) => line !== "" && !line.startsWith("#"));
  if (procfile === null) {
    problems.push('Add a Procfile with the command that starts the server, such as "web: gunicorn app:app --bind 0.0.0.0:$PORT".');
  } else if (web.length !== 1 || !/^web\s*:\s*\S/.test(web[0])) {
    problems.push('The Procfile needs exactly one line, "web: <command>". Sprout runs only a web process.');
  }
  return problems;
}

async function readText(files: SourceFile[], path: string): Promise<string | null> {
  const file = files.find((candidate) => candidate.path === path);
  if (!file || file.size > MAX_FILE_BYTES) return null;
  return new TextDecoder().decode(await file.read());
}

/** Reads requirements.txt and the Procfile from a prepared folder and describes any problems. */
export async function inspectPythonApp(files: SourceFile[]): Promise<string[]> {
  return describePythonProblems(await readText(files, "requirements.txt"), await readText(files, "Procfile"));
}
