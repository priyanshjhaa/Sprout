# Node.js application contract — version 1

This is the first supported application type for Sprout. A local smoke runner executes the reference app's build command inside an offline, disposable Node container. It does not export a build image, start an application runtime, or connect to simulation endpoints. Simulation jobs remain simulations.

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

The local smoke runner executes the dependency and build rows. The runtime row belongs to a later milestone:

| Phase | Command | Boundary |
| --- | --- | --- |
| Dependencies | `npm ci --offline --ignore-scripts --no-audit --no-fund` | Local smoke runner: dependency-free lockfile, offline container |
| Build | `npm --ignore-scripts run build` | No network; bounded disposable container |
| Runtime | `npm --ignore-scripts start` | Separate runtime container with explicit routing and permissions |

The smoke runner uses `node@sha256:0e0ff40c39bc087845bfb27465a0df4ea419520094bc35842ff83dd8cbe6f9b6`, the official `node:24.21.0-bookworm-slim` image digest downloaded for this milestone. Docker identified it as Linux/arm64 on this host. The runner does not use a moving `node:24` tag. The application cannot select the image, override container flags, mount host paths, or request a Docker socket.

The application must listen on `0.0.0.0` using `PORT` (initially `3000`), return HTTP 200 from `/health` when ready, and shut down on SIGTERM. These are runtime acceptance conditions; static JSON validation cannot prove them. No real URL is reported until readiness and routing succeed.

## Current local build isolation

The local smoke runner uses a disposable non-root container with a read-only root filesystem, all Linux capabilities dropped, no-new-privileges, Docker's default seccomp profile, no networking, and no host mounts. It does not use privileged mode, host namespaces, the Docker socket, or inherited host environment variables. Docker control access belongs only to trusted Sprout platform code. The command is local developer tooling and assumes the configured Docker context is a trusted local daemon.

The local runner applies a two-minute timeout, one CPU, 1 GiB memory with no extra swap, 128 processes, a 512 MiB workspace tmpfs, and 64 MiB `/tmp`. It discards container logs and exports no build output. These are local smoke-build limits, not shared service capacity controls.

The local runner uses `npm ci --offline` and accepts only lockfiles with no third-party dependencies. The build runs `npm --ignore-scripts run build` with networking disabled. Arbitrary dependency downloads remain unsupported until constrained egress, redirects, cache ownership, and registry policy are enforced. Do not weaken network isolation just to make an install pass.

Cancellation interrupts the Docker client, then removes only this run's specifically named container with a separate bounded cleanup context. A short poll handles the race where the daemon creates the container as cancellation arrives. Normal completion and build failure also remove that container. The runner never prunes unrelated containers, volumes, images, or caches. Crash recovery and storage accounting must be defined before public build submission.

Docker restrictions are defense in depth, not proof of safe hostile multi-tenant execution. Shared-kernel risks remain. Production use needs a dedicated, appropriately isolated build environment and security review; this local development milestone does not certify that boundary.

## Current request/resource trace

`sourcecheck -build-node` → bounded archive extraction → root-confined filesystem → Node contract and dependency-free checks → bounded tar input → trusted Docker CLI → pinned, network-disabled build container → safe summary/error → named-container cleanup → temporary-tree cleanup.

`sourcecheck -runtime node` remains validation-only. `-build-node` executes the submitted build script inside Docker; it does not export `dist`, create a deployment image, start the runtime, or introduce new tables, credentials, HTTP endpoints, or frontend changes.

## Verification

From `backend/`, create an archive of only the explicit example files in a directory you own:

```sh
tar --format=ustar -cf /tmp/sprout-node-example.tar -C dev/node-example package.json package-lock.json build.mjs server.mjs
go run ./cmd/sourcecheck -archive /tmp/sprout-node-example.tar -runtime node
go run ./cmd/sourcecheck -archive /tmp/sprout-node-example.tar -build-node
SPROUT_DOCKER_TEST=1 go test ./internal/nodeapp -run '^TestDockerSmokeRunner$' -count=1 -v
```

The example has no dependencies. Its build copies the server into `dist`; its start command is reserved for the separate runtime milestone. Avoid running submitted application scripts directly on your host, even when metadata validation succeeds.

Tests cover required scripts, Node version, npm/workspace policy, root/lock mismatches, dependency URLs and integrity shape, linked packages, source-tree exclusions, malformed/oversized JSON, cancellation, and the committed example. The CLI reports only source counts or safe error codes, never script contents or package/provider error payloads.

The `-build-node` command requires Docker to be running and the pinned Node image to already be present locally; it does not pull images automatically. It reports success/failure without printing submitted build output. The four-file reference app completed the restricted build and reported `4` files and `1,162` source bytes. The opt-in Docker test passed success, build-failure, and cancellation paths; each left no labeled build container behind. Application runtime, build output export, and production isolation remain future work.

## Learning checkpoint

The mental model is a trusted supervisor giving untrusted build code a small disposable workbench. The Go process validates and prepares the input, starts Docker with fixed permissions and limits, waits for the result, and cleans up the one container it created. It does not make the application code trustworthy.

In NestJS or Django, the same product flow would usually be implemented by an API handler handing work to another process or job system. Here, the local Go command directly supervises the Docker CLI with a request context and an explicit cleanup path. That is useful for learning process and cancellation mechanics, but this local command is not yet the eventual asynchronous deployment worker.

1. Why can a valid package manifest still contain a dangerous build script?
2. Why does disabling install hooks not make `npm run build` harmless?
3. Why must cancellation remove the container rather than merely stop the Docker CLI?
4. Which part of this flow is the trusted supervisor, and which part is untrusted code?

Commit boundary: `feat: add restricted Node build smoke runner`.

## References

- [Node.js release lines](https://nodejs.org/en/about/previous-releases)
- [npm ci and ignore-scripts behavior](https://docs.npmjs.com/cli/commands/npm-ci/)
- [Docker security boundary](https://docs.docker.com/engine/security/)
- [Docker default seccomp profile](https://docs.docker.com/engine/security/seccomp/)
