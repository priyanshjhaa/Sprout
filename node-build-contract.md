# Node.js application contract — version 1

This is the first supported application type for Sprout. The current implementation validates prepared source and provides a small reference app. **It does not yet install dependencies, execute builds, create images, or deploy applications.** Simulation endpoints stay unchanged.

## Accepted source

- A single npm application at the archive root, with `package.json` and `package-lock.json`.
- `private: true`, a nonempty name/version, and `engines.node: "24.x"`.
- Nonempty `scripts.build` and `scripts.start` (at most 2,048 bytes each).
- npm lockfile version 3, with matching application name/version and root dependency maps.
- Every direct dependency must have a corresponding lock entry. Locked packages must have a public `https://registry.npmjs.org` tarball URL and one SHA-512 integrity digest.
- No Git/file/URL dependency specifiers, linked packages, workspaces, alternative package-manager lockfiles, uploaded `node_modules`, `.next`, or `dist` directories.
- No submitted Dockerfile or `.dockerignore`. Sprout—not source code—will select the image and container settings.
- Existing archive path, size, credential-file, and cleanup restrictions still apply. Manifest limit: 256 KiB; lockfile limit: 1 MiB; maximum 4,096 lock entries.

This initial policy is intentionally narrower than npm. Native dependencies requiring install hooks, private registries, monorepos, and framework-specific runtimes are not supported yet. Extra npm lifecycle scripts may exist, but the controlled commands below disable automatic pre/post and install hooks.

The Go validator is a preflight check, not a reimplementation of npm's dependency resolver. `npm ci` must still validate the complete dependency graph. Integrity strings are checked for shape here; npm must verify downloaded content against them during installation. A registry URL allowlist is not an egress firewall or a guarantee that package code is safe.

## Platform-owned commands

The following are the agreed execution contract, **not commands currently run by sourcecheck**:

| Phase | Command | Boundary |
| --- | --- | --- |
| Dependencies | `npm ci --ignore-scripts --no-audit --no-fund` | Restricted dependency-fetch environment; no credentials or general network access |
| Build | `npm --ignore-scripts run build` | No network; bounded disposable container |
| Runtime | `npm --ignore-scripts start` | Separate runtime container with explicit routing and permissions |

Node 24 and the bundled npm version will be pinned through a reviewed image digest before execution is enabled. A moving `node:24` tag is not sufficient for reproducible builds. The application cannot select the image, override container flags, mount host paths, or request a Docker socket.

The application must listen on `0.0.0.0` using `PORT` (initially `3000`), return HTTP 200 from `/health` when ready, and shut down on SIGTERM. These are runtime acceptance conditions; static JSON validation cannot prove them. No real URL is reported until readiness and routing succeed.

## Build isolation requirements for the next implementation

Use a disposable non-root container with a read-only root filesystem, all Linux capabilities dropped, no-new-privileges, and the default seccomp profile. Do not use privileged mode, host networking/PID namespaces, host bind mounts, the Docker socket, or inherited host environment variables. Docker control access belongs only to trusted Sprout platform code.

Initial build budgets: one CPU, 1 GiB memory with no additional swap, 128 processes, a size-limited 512 MiB writable temporary filesystem, and a two-minute wall-clock timeout. Treat these as proposed defaults to verify with the reference app, not implemented protections. Container output must be bounded and must not be persisted or printed as unrestricted raw logs.

Separate dependency acquisition from script execution. The first isolated smoke test will use the dependency-free reference app with networking disabled. Do not enable arbitrary dependency downloads until constrained egress, redirects, cache ownership, and registry policy are enforced. Do not weaken network isolation just to make an install pass.

Cancellation must stop and remove the owned container—not only terminate the Docker client. Use explicit ownership labels and identifiers for cleanup; never prune unrelated containers, volumes, images, or caches. Cleanup gets its own bounded context after a cancelled build. Crash recovery and storage accounting must be defined before public build submission.

Docker restrictions are defense in depth, not proof of safe hostile multi-tenant execution. Shared-kernel risks remain. Production use needs a dedicated, appropriately isolated build environment and security review; this local development milestone does not certify that boundary.

## Current request/resource trace

`sourcecheck -runtime node` → bounded archive extraction → root-confined filesystem → bounded manifest/lock reads → contract checks → safe summary → temporary-tree cleanup.

No JavaScript is executed during this trace. No new tables, credentials, HTTP endpoints, or frontend changes are introduced.

## Verification

From `backend/`, create an archive of only the explicit example files in a directory you own:

```sh
tar --format=ustar -cf /tmp/sprout-node-example.tar -C dev/node-example package.json package-lock.json build.mjs server.mjs
go run ./cmd/sourcecheck -archive /tmp/sprout-node-example.tar -runtime node
go test -race ./internal/nodeapp ./internal/source ./cmd/sourcecheck
go vet ./...
```

The example has no dependencies. Its build copies the server into `dist`; its start command will launch that server when container execution is implemented. Avoid running submitted application scripts directly on your host, even when metadata validation succeeds.

Tests cover required scripts, Node version, npm/workspace policy, root/lock mismatches, dependency URLs and integrity shape, linked packages, source-tree exclusions, malformed/oversized JSON, cancellation, and the committed example. The CLI reports only source counts or safe error codes, never script contents or package/provider error payloads.

Verification for this milestone: full backend tests with race detection and local PostgreSQL integrations, `go vet`, and repository lint passed. Manual archive validation accepted the four-file example (`1,162` source bytes) and rejected an archive missing its manifest with `node_contract_invalid`. The example scripts were not executed; container build and runtime verification remain future work.

## Learning checkpoint

Go is currently acting as an inspector: reading JSON and enforcing platform rules. Later it will act as a process/container supervisor. NestJS or Django could enforce the same contract; Go makes filesystem ownership, cancellation, and process cleanup explicit instead of supplying a deployment framework.

1. Why can a valid package manifest still contain a dangerous build script?
2. Why does disabling install hooks not make `npm run build` harmless?
3. Why must cancellation remove the container rather than merely stop the Docker CLI?

Commit boundary: `feat: validate Node application build contract`.

## References

- [Node.js release lines](https://nodejs.org/en/about/previous-releases)
- [npm ci and ignore-scripts behavior](https://docs.npmjs.com/cli/commands/npm-ci/)
- [Docker security boundary](https://docs.docker.com/engine/security/)
- [Docker default seccomp profile](https://docs.docker.com/engine/security/seccomp/)
