# Backend systems story: why Sprout is built this way

This is the first systems-story lesson. It covers backend milestones 0–8 as they exist today. It explains *why* each part exists, not how to type it. Every chapter follows the same shape:

**Problem → History → Decision in Sprout → Trade-off → Check**

Future milestones add chapters in the same format.

---

## Chapter 0: You are building a control plane

**Problem.** Running someone's application involves two jobs: actually serving its traffic, and deciding what runs, where, and who may touch it.

**History.** Every cloud splits these into two halves. The **data plane** is the user's apps serving requests. The **control plane** is the system that manages them. Heroku (2007) made `git push` → URL feel magical by hiding the control plane completely. Kubernetes (2014) made the control plane explicit and famously complex.

**Decision in Sprout.** The Go API in `backend/` is the control plane. Everything built so far (applications, identity, sharing, deployment jobs, progress streaming) is control plane. Milestone 8 is where Sprout first touches the data plane: build containers, and later runtime containers.

**Trade-off.** Keeping both planes in one process is simple now. A real runtime must be able to survive the API restarting.

**Check.** If the Go API crashes, should a deployed application stop serving traffic?

## Chapter 1: Why Go, and why one binary

**Problem.** A deployment platform spends most of its life *waiting*: on network connections, subprocesses, timers and databases. It must supervise many of these at once without becoming fragile.

**History.** Google started Go in 2007 because large C++ servers compiled slowly and struggled with huge numbers of concurrent connections. The language made cheap concurrent supervision a first-class idea. That is why Docker, Kubernetes, Terraform and Caddy are written in Go: they are all programs that supervise other programs.

**Decision in Sprout.** One compiled executable (`cmd/api`) with a small runtime inside it. That runtime contains a scheduler, which multiplexes many goroutines onto a few OS threads, and a garbage collector. There is no separate interpreter to install.

**Trade-off.** Node runs your JavaScript on one thread with an event loop, so shared memory rarely races. Go goroutines can truly run in parallel, so Go makes you think about races, locks and ownership explicitly. That is why `go test -race` is part of every concurrent milestone.

**Check.** What is still inside a compiled Go program at runtime, and why does it need a scheduler?

## Chapter 2: A process has a life (`cmd/api/main.go`)

**Problem.** Servers are started, restarted and stopped by machines (systemd, Docker, Kubernetes), not people. A process must communicate clearly with them, through exit codes, signals and health endpoints.

**History.**
- Orchestrators stop a process politely first. They send SIGTERM ("please finish"), wait (30 seconds by default in Kubernetes), then send SIGKILL (instant death).
- Load balancers later learned to ask two different questions: "Are you alive?" (if not, restart) and "Can you take traffic?" (if not, route around).

**Decision in Sprout.** Read `main.go` as the biography of the process:
1. `config.Load` validates configuration before anything else and exits with code 1 on failure. A typo in `DATABASE_URL` should stop startup, not cause a failure on the first request at 3 a.m.
2. `signal.NotifyContext(..., SIGTERM)` turns the shutdown signal into a cancelled `context`, which flows into every layer.
3. Construction follows the dependency graph: config → database pool → worker lock → worker → services → router.
4. `defer` runs in reverse order, so shutdown also runs in reverse: stop HTTP → stop workers → close the pool. The database stays open long enough for workers to record "cancelled."
5. `/health/live` answers "alive?" and `/health/ready` answers "can I take traffic?", checking the database, worker health and lock ownership.

**Trade-off.** Explicit wiring is more lines than a dependency-injection framework, but every lifetime is visible in one file.

**Check.** Why must the database pool close *after* the workers, not before?

## Chapter 3: HTTP is a hostile network boundary

**Problem.** Anyone on the network can open a connection and behave badly: sending slowly, sending too much, or triggering a bug.

**History.** Slowloris (2009) took down Apache servers by holding thousands of connections open and sending headers one byte at a time. Servers learned to set read, write and idle timeouts on every connection.

**Decision in Sprout.**
- `net/http` gives each connection its own goroutine, which is cheap, and `httpapi/server.go` sets read-header (5s), read (10s), write (30s) and idle (60s) timeouts so slow clients can't hold them forever.
- Middleware wraps every handler like an onion (`httpapi/server.go`): CORS → request ID → request logging → panic recovery, with authentication added for `/api/v1` in `main.go`.
- **Request IDs:** when one user action touches the API, the database and a worker, a shared ID is the only way to stitch the logs back together.
- **Panic recovery:** a bug in one handler becomes a safe `500` instead of killing everyone's connections.
- All errors share one envelope with stable codes, so clients never see SQL or stack traces.

**Trade-off.** Recovering from a panic is not the same as handling an error. The bug still exists; recovery only contains the damage.

**Check.** Who creates the goroutine that runs your handler, and when is its context cancelled?

## Chapter 4: The database is the source of truth

**Problem.** Product state must survive restarts and stay consistent when many requests write at once.

**History.**
- PostgreSQL forks a whole OS process per connection, costing megabytes of memory each. Applications learned to share a small **pool** of connections instead of opening one per request.
- ORMs became popular for convenience, but they generate SQL you never read. N+1 query bugs and missing tenant filters hide inside that generated SQL.

**Decision in Sprout.**
- `internal/database/pool.go` opens a bounded `pgxpool`. Pool size is the first piece of backpressure: when every connection is busy, new work waits instead of crushing PostgreSQL. A cancelled request makes `pgx` send PostgreSQL a cancel message, so abandoned queries stop using CPU.
- SQL lives in `backend/queries/*.sql`, where it can be reviewed. `sqlc` generates typed Go code from it (`internal/database/dbgen`).
- Drizzle is the **only** schema and migration owner. Two systems that both "own" migrations eventually disagree about what the database looks like.

**Trade-off.** Writing SQL by hand is more work than calling an ORM, and the schema lives in TypeScript while the queries live in Go. In exchange, every query that touches tenant data can be read and reviewed.

**Check.** What happens to a new request when every pooled connection is busy?

## Chapter 5: Multi-tenancy, the bug that leaks companies

**Problem.** Many workspaces share one database. One missing filter shows company A's apps to company B.

**History.** The most common SaaS security bug has a name: **IDOR** (insecure direct object reference). `GET /apps/123` returns app 123 to anyone who guesses the number. Separately, password-reset systems learned to store only token *hashes*, so a leaked database contains no usable keys.

**Decision in Sprout.**
- Clerk answers **who you are**. PostgreSQL answers **what you may touch**. A valid session alone grants nothing (`authorization-policy.md`).
- Every application query is scoped by workspace *and* user (`queries/applications.sql`, `queries/access.sql`).
- Hidden or cross-workspace resources return `404`, not `403`, so Sprout never confirms that something exists to people who can't see it.
- Invitation tokens are stored only as a SHA-256 digest (`internal/team`).
- Team changes lock the workspace row in PostgreSQL. They lock in the database, not with a Go mutex, because a mutex protects only one process while a database lock protects every process using that database.
- Accepting an invitation and creating the membership commit in one transaction: both happen, or neither does.

**Trade-off.** Authorization checks repeat inside SQL rather than living in a single middleware. That repetition is deliberate.

**Check.** Why must a restricted application's query include both the workspace and the requesting user?

## Chapter 6: Work that outlives the request (`internal/deployment/worker.go`)

**Problem.** A build takes minutes, and an HTTP request should take milliseconds. If the job lives inside the request, closing the browser kills the deployment.

**History.** Every web ecosystem hit this problem and grew a job system: Sidekiq for Ruby, Celery for Python, BullMQ for Node. Teams also learned that unbounded queues don't remove overload; they hide it until memory runs out.

**Decision in Sprout.**
- 2 workers, a waiting room (buffered channel) of 8, and at most one accepted job per application.
- **Full queue:** returns `503` with `Retry-After`. That is backpressure, telling the caller the honest truth.
- **The job is saved before it goes on the channel.** The channel lives in memory and dies with the process. The PostgreSQL row is the record of truth; the channel is only a fast way to hand work to a worker.
- **A PostgreSQL advisory lock** makes exactly one process own the workers, so two servers never run the same job.
- **After a crash**, unfinished jobs become `process_interrupted` instead of being replayed. A half-finished deployment is not safe to blindly re-run. This is the classic at-least-once vs at-most-once trade-off, and Sprout chose honesty over magic.

**Trade-off.** In-process workers are simple but not durable or distributed. One API process per database schema is a known limit.

**Check.** Why is the deployment saved in PostgreSQL *before* being placed on the channel?

## Chapter 7: Streaming progress (`httpapi/deployment_stream.go`)

**Problem.** Users want to watch a deployment progress without refreshing the page.

**History.** There are three common answers:
- **Polling:** simple, but wasteful.
- **WebSockets:** two-way, but stateful and complex.
- **Server-Sent Events (SSE):** plain one-way HTTP. It fits when updates only flow from server to browser.

**Decision in Sprout.** SSE, built as a **doorbell, not a storage box**. The worker saves the change, then rings. If the browser is slow, five rings collapse into one, and the stream reads the latest state from PostgreSQL. One slow browser can never block a deployment. Each heartbeat re-checks permission, so revoked access eventually reaches long-lived connections too. Limits: 4 streams per user, 128 per process, 25-second connections.

**Trade-off.** Intermediate states can be skipped, and there is no replay log. That is acceptable for progress, but not for audit history.

**Check.** Why does closing the progress page stop the listener but not the deployment?

## Chapter 8: Running strangers' code, which is the USP

**Problem.** Sprout's whole promise is running code it did not write and cannot trust. That code may try to steal secrets, attack the host, starve other tenants, or exploit Sprout's own extraction and build tools.

**History.**
- **chroot (1979) → FreeBSD jails (2000):** early ways to fence off part of a machine.
- **Linux namespaces and cgroups (2008):** namespaces control what a process can *see*; cgroups control how much it can *use*.
- **Docker (2013):** made those two features easy to use.
- **The catch:** containers share the host kernel, so one kernel bug can become an escape. That is why AWS built **Firecracker** microVMs (2018) for Lambda, and why Google built **gVisor**.
- **Zip Slip (2018):** archives containing `../../` paths overwrote files across thousands of projects.
- **The event-stream npm attack (2018):** malicious code hidden in a popular dependency.

**Decision in Sprout.**
- `internal/source/archive.go` extracts only plain files through a root-confined `os.Root`. It rejects traversal paths, links, devices, sensitive filenames and oversized input.
- `internal/nodeapp/contract.go` accepts a narrow Node 24 application shape before anything runs.
- `internal/nodeapp/runner.go` builds inside Docker, where each flag answers a specific attack:

| Flag | Attack it stops |
|---|---|
| `--network none` | Steal secrets and send them out, or attack the internal network |
| `--read-only`, `--cap-drop ALL`, `no-new-privileges`, user `65532` | Escalate privileges or modify the image |
| `--pids-limit 128` | Fork bomb |
| `--memory 1g`, `--cpus 1` | Starve other tenants' apps |
| Timeout + named-container cleanup | Run forever, or leave orphaned containers behind |
| `--pull=never` + image pinned by digest | A moving tag silently changing the build image |
| `npm ci --offline --ignore-scripts` | Install-time scripts from a compromised dependency |

- `internal/artifact/store.go` writes to a temporary file, syncs it, then renames it into place. On POSIX, rename is atomic, so a reader sees either no artifact or a complete one, never half of one. A SHA-256 digest is checked on read.

**Trade-off.** Docker restrictions are defence in depth, not proof of safety for hostile multi-tenant workloads. Production needs a stronger isolation boundary and a security review.

**Check.** Why must cancellation remove the container, rather than only stopping the Docker CLI process?

---

## What's next: the data plane

Next comes a runtime container, a `/health` probe, a reverse proxy, and finally a real URL. After that come the economics that define Sprout: many small, low-traffic apps are only affordable if idle apps release their resources and wake on demand. Google Cloud Run and Fly.io make small apps cheap this way. In Sprout, pause and resume aren't settings toggles; they are the business model.
