# Real build jobs

This continues backend milestone 8. It turns the pieces that already existed (safe source preparation, the sealed Node build, and the artifact store) into deployment jobs that run on the existing bounded worker. It does **not** start the built application, check its health, or give it a URL; that is the runtime milestone.

The work is split into independently committed slices:

1. **Artifact references** (`feat: reference real build artifacts from deployments`): deployments can record the ID, SHA-256 and size of a build's output. The bytes stay in the local artifact store.
2. **One worker, two kinds of job** (`refactor: run real and simulated deployments on one worker`): the manager runs any job through a `Runner`; simulations become one runner among others.
3. **Source intake** (`feat: accept source uploads for local builds`): an upload endpoint behind a local-only flag, a private source store, a real build runner, and a startup sweep for orphaned files.
4. **Dashboard upload** (`feat: upload and follow real builds from the dashboard`): a capabilities endpoint, an upload control on the Deploy page and Deployments tab, and live build progress.

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

## Slice 3: source intake

**Problem.** Accepting code over HTTP is the moment Sprout can be attacked by
anyone who can reach it. Every byte received costs disk, every queued build costs
CPU, and every crash can strand files.

**History.** Upload services learned to answer cheap questions before expensive
ones: S3 and most CDNs reject unauthorized or oversized requests from headers
alone, so a refused client never gets to stream gigabytes. Build systems that
keep scratch files (CI runners, Bazel's output base) all grew a sweep at startup,
because a killed process never runs its cleanup code.

**Decision.**

- **Off by default.** Builds exist only with `SPROUT_ENABLE_LOCAL_BUILDS=1`, and
  config refuses to start unless the API listens on a loopback address. Without
  the flag the `/deployments` paths do not exist. Uploaded code runs in Docker on
  this machine; that must never be offered to anyone else.
- **Cheap checks first.** Permission, worker health, the per-app slot and queue
  space are checked before the body is read. A refused request costs no disk.
- **Bounded input.** Only `application/x-tar`, at most 16 MiB (`413` beyond),
  then the existing extraction limits and the Node contract.
- **Sources are disposable.** The upload is saved under a random ID in a private
  store, handed to the job in memory, and deleted when the job ends. It never
  appears in the database.
- **Disk is bounded by design.** At most 2 running and 8 queued jobs exist, so at
  most 10 uploads (160 MiB) are ever stored at once.
- **Startup sweep.** After the worker lock is held and interrupted jobs are
  failed, every stored upload and every artifact no deployment references is
  removed, along with partial writes.
- **Stable failure codes.** The runner maps known errors (`node_contract_invalid`,
  `source_sensitive_path`, `node_build_failed`, …) to codes; anything else becomes
  `build_failed`.

**Trade-off and known limits.**

- Artifacts of succeeded builds are kept indefinitely. A retention rule (for
  example, keep the latest few per application) must exist before builds are
  offered beyond this machine.
- Docker restrictions are defence in depth on a shared kernel, not proof of safe
  hostile multi-tenant execution (see `node-build-contract.md`).
- Dependency installs are still offline-only, so only dependency-free apps build.

## Slice 4: dashboard upload

**Problem.** The dashboard must offer uploads only when the API can build, and
must never confuse a real build with a simulation.

**History.** Clients that infer features from failures (a 404 here, a timeout
there) break the moment an unrelated error looks the same. APIs such as
Kubernetes discovery or OAuth server metadata publish what they support instead.

**Decision.**

- `GET /api/v1/capabilities` returns `{"localBuilds": bool}`. The dashboard, a
  future CLI and agents ask instead of guessing.
- With builds on, the Deploy page offers **Upload & build** after creating an app
  (a simulation stays available), and the Deployments tab lists builds above
  simulations, each with its own upload or run control.
- One client handles both kinds by path, and the response parser takes the kind
  it expects, so a simulation view rejects a real build and vice versa.
- Build detail pages (`?kind=build`) stream live progress like simulations and
  explain failure codes in plain language (for example, a `.env` in the archive).

## Request and job trace

```text
POST …/deployments (application/x-tar)
  → authenticate → CheckBuild: permission, worker health, app slot, queue space
  → read body, capped at 16 MiB (413 beyond)
  → save source blob (private store, random ID)
  → SubmitBuild: re-check admission, persist queued row, enqueue
       (if refused: delete the source blob before answering)
  → worker: Start (recheck access, lifecycle)
  → source:  load blob (digest checked) → extract to private temp tree → Node contract
  → build:   sealed Docker build of the tree → bounded dist/ tar
  → package: save artifact blob
  → Finish (status, code, artifact reference only if succeeded and not cancelled)
  → Release: delete source blob; delete artifact unless recorded
```

## Verification

From `backend/` with the local database environment loaded:

```sh
go test -race ./...
go vet ./...
SPROUT_DOCKER_TEST=1 go test -run TestDockerBuildJob -v ./internal/buildjob
```

Recorded for this milestone: the full race suite with PostgreSQL passed; the
Docker build job test built the committed example, stored its `dist/` artifact,
and Docker events showed the build container created and destroyed. Starting the
API with local builds enabled swept a planted partial upload (`removed: 1`),
logged the local-builds warning, and answered unauthenticated uploads with `401`.
After slice 4, frontend typecheck, lint, production build and stream tests passed,
and `/api/v1/capabilities` required a session. A signed-in upload through the
dashboard requires a real Clerk session and is checked manually:

1. Start the API with local builds enabled (see `.env.example`).
2. From `backend/`, create the example archive:
   `tar --format=ustar -cf /tmp/sprout-node-example.tar -C dev/node-example package.json package-lock.json build.mjs server.mjs`
3. In the dashboard, create an app on Deploy, choose that file, and press
   **Upload & build**. Expect source, build and package to succeed and the build
   to end as Succeeded with "Built and stored".
4. Add a `.env` file to a copy of the archive and upload it: expect a failed build
   that explains the sensitive file, and no artifact left in the artifact folder.

## Explain-back checkpoint

1. Why is `Release` called after `Finish` rather than at the end of `Run`?
2. A build finishes at the same moment a user cancels it. Which one wins in the database, and what happens to the artifact?
3. Why would storing the runner's raw error text as the failure code be a security problem?
4. Why does the upload handler check permission and queue space before reading the body?
5. Why must the startup sweep run only after the worker lock is acquired?
