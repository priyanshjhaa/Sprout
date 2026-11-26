import assert from "node:assert/strict";
import test from "node:test";

import { describePythonProblems } from "../src/lib/python-readiness.ts";

const hash = "a".repeat(64);

test("a hashed requirements.txt and a web Procfile are ready", () => {
  const requirements = `# generated\nflask==3.1.3 \\\n    --hash=sha256:${hash}\n    # via -r requirements.in\n`;
  assert.deepEqual(describePythonProblems(requirements, "web: gunicorn app:app --bind 0.0.0.0:$PORT\n"), []);
  assert.deepEqual(describePythonProblems("# just the standard library\n", "web: python app.py"), []);
});

test("unpinned or unhashed requirements are explained", () => {
  assert.match(describePythonProblems("flask\n", "web: x")[0], /pinned with ==/);
  assert.match(describePythonProblems("flask==3.1.3\n", "web: x")[0], /hashes/);
  assert.match(describePythonProblems("-r base.txt\n", "web: x")[0], /options/);
});

test("the Procfile must have exactly one web line", () => {
  const requirements = `flask==3.1.3 --hash=sha256:${hash}\n`;
  assert.match(describePythonProblems(requirements, null)[0], /Add a Procfile/);
  assert.match(describePythonProblems(requirements, "worker: python job.py\n")[0], /exactly one line/);
  assert.match(describePythonProblems(requirements, "web: a\nworker: b\n")[0], /exactly one line/);
});
