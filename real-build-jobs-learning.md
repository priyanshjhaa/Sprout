# Real build jobs

This continues backend milestone 8. It turns the pieces that already existed (safe source preparation, the sealed Node build, and the artifact store) into deployment jobs that run on the existing bounded worker. It does **not** start the built application, check its health, or give it a URL; that is the runtime milestone.

The work is split into independently committed slices:

1. **Artifact references** (`feat: reference real build artifacts from deployments`): deployments can record the ID, SHA-256 and size of a build's output. The bytes stay in the local artifact store.
2. **One worker, two kinds of job** (`refactor: run real and simulated deployments on one worker`): the manager runs any job through a `Runner`; simulations become one runner among others.
3. **Source intake** (next): an upload endpoint behind a local-only flag, a private source store, and a startup sweep for orphaned files.
4. **Dashboard upload** (next): the Deploy page's archive upload becomes real.

## Slice 1: artifact references

**Problem.** A real build produces bytes that later steps (release, rollback) must find and trust. Storing them in PostgreSQL would bloat backups and mix untrusted output with product state.

**History.** Docker images are referenced by digest, not by tag, because a name can be repointed but a digest names exactly one content. Heroku kept "slugs" in object storage and only a reference in its database.

**Decision.** `deployments` gains `artifact_id`, `artifact_sha256` and `artifact_bytes`. A check constraint accepts them only as a complete set, only on a non-simulated row, and only when the row `succeeded`. The one-active-job index now covers both kinds, because both share one worker.

**Trade-off and lesson.** The first version of the constraint was wrong: in SQL, `NULL ~ 'pattern'` is `NULL`, and a `CHECK` treats `NULL` as passing, so a half-written reference slipped through. A direct probe caught it, and every column is now tested with `is not null`. Constraint logic deserves the same failure-path testing as Go code.

## Slice 2: one worker, two kinds of job

**Problem.** The manager knew how to wait in place of work, one stage at a time. A real build owns resources across stages (an extracted tree, a container, an output file) and must clean them up however the job ends.

**History.** Build systems separate the *scheduler* (what runs, when, with what limits) from the *executor* (how to do the work). Kubernetes Jobs and CI runners both work this way, because scheduling rules such as capacity, deadlines and cancellation should not be rewritten for every kind of work.

**Decision.**

- `Runner` has two methods. `Run` does the work and reports stage boundaries through `Steps`, which the manager persists and streams. `Release` is called exactly once, after the final state is saved, and frees what the job owned. Its `kept` flag says whether the artifact is now referenced and must survive.
- An artifact is recorded only if the job succeeded and the database accepted the result. If cancellation won the race, `Finish` reports that nothing was recorded, and `Release` deletes the artifact.
- Runner errors become failure codes only through `Failure{Code}` with an allow-listed shape. Any other error is stored as `build_failed` (or `simulated_step_failed`), so raw tool output never reaches the database or API.
- A panic in `Release` is contained, so it cannot kill a worker goroutine other jobs depend on.
- `Scope.Simulated` selects which kind of deployment a caller sees. The simulation endpoints set it, so they never return real builds, and vice versa.
- Crash recovery now fails unfinished real builds too (`process_interrupted`), because the worker owns them.

**Trade-off.** Cleanup is still in-process. If the process is killed, `Release` never runs; the startup sweep in slice 3 removes whatever was left behind.

## Request and job trace (after slice 2)

```text
Submit → authorize → persist queued row (simulated or real) → bounded channel
  → worker: Start (recheck access, lifecycle)
  → Runner.Run: Begin/Complete per stage (persisted, streamed)
  → Finish (status, code, artifact only if succeeded)
  → Runner.Release (delete source; delete artifact unless kept)
```

## Explain-back checkpoint

1. Why is `Release` called after `Finish` rather than at the end of `Run`?
2. A build finishes at the same moment a user cancels it. Which one wins in the database, and what happens to the artifact?
3. Why would storing the runner's raw error text as the failure code be a security problem?
