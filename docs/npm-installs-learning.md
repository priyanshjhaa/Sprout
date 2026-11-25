# npm installs: real Node apps and built websites

Until now Sprout only built Node apps that used **no npm packages**. That ruled out almost every real project: an Express server, a Vite or React site, anything an agent writes. This milestone lets apps use packages from the public npm registry. It also lets a Node project with no `start` script build a **website**, which is served from its `dist/` folder.

Commits: `feat: fetch locked npm packages into a verified cache`, `feat: install npm packages in the sealed offline build`, `feat: serve a website built from a Node project`, `feat: tell every front door that npm packages work`, plus these docs.

## The mental model

The build container still has **no network**. Sprout downloads the packages *for* it, checks each one against the lockfile's checksum, and hands them in. Untrusted code and the internet never meet.

```
package-lock.json ──► Sprout (trusted Go) ──► registry.npmjs.org        only this host, only the locked tarballs
                         │  checks SHA-512 against the lockfile
                         ▼
                   package cache (by digest) ──► sealed build container   --network none
                                                   npm cache add → npm ci --offline --ignore-scripts
                                                   → npm run build → npm prune --omit=dev
                                                   ──► release: dist/ + package.json + node_modules
                                                   (or, with no start script: just the website in dist/)
```

The plan said "a controlled registry proxy". An experiment at the start of this milestone showed something better was possible: if npm finds every tarball already in its cache, `npm ci --offline` never needs a network at all. So no proxy, no filtered network and no firewall rules. The build simply has nothing to connect to.

## Story 1: Why installing packages is the dangerous part

**Problem.** `npm install` does two risky things. It downloads code from the internet, and it can **run** code: packages may declare `preinstall`, `install` and `postinstall` scripts that run the moment the package lands. A build that can reach the internet can also leak whatever it can read, or download anything at all.

**History.** Attacks through the package registry became a defining security story of the 2010s and 2020s:
- In 2018, `event-stream` was handed to a stranger who added code that stole cryptocurrency wallets.
- In 2021, the `ua-parser-js` maintainer's account was hijacked and the package shipped a cryptominer through an install script.
- "Dependency confusion" attacks tricked build systems into downloading a public package instead of a company's private one.

The industry's answers came in layers:
- **Lockfiles** pin exact versions and record a cryptographic checksum (`integrity`) for every tarball.
- **`npm ci`** installs exactly what the lockfile says, or fails.
- **`--ignore-scripts`** never runs install scripts.
- **Hermetic builds:** systems like Google's Bazel and Nix fetch everything first, verify it, then build with no network at all, so a build can neither leak data nor depend on what the internet says today.

**Decision in Sprout.** Use all four layers:
- **Lockfile and checksums:** `package-lock.json` is required, and every entry must point at `registry.npmjs.org` with a SHA-512 checksum.
- **Install exactly the lockfile:** the build runs `npm ci --offline --ignore-scripts`.
- **A hermetic build:** the container has `--network none`, as before.

**Trade-off.** Packages that *need* their install script fail: native modules that compile on install, such as `bcrypt`, `sharp` or `sqlite3`. They get their own failure code (`node_build_install_failed`) and a plain explanation. Modern tools such as esbuild, rollup and Vite deliver ready-made binaries as separate per-platform packages instead, and those work.

## Story 2: Sprout as the trusted downloader

**Problem.** Something must download the tarballs, and that something touches the internet. It must not be the untrusted build.

**History.** This is the "fetch phase" of hermetic build systems. Nix and Bazel name every input by its hash, download it once, verify it, and keep it in a store keyed by that hash. Because the name *is* the checksum, a cached file can be trusted forever and shared by every build that needs the same bytes.

**Decision in Sprout** (`backend/internal/npmfetch`).
- **Reading the lockfile:** the fetcher takes every entry this platform needs. It skips packages built for other systems, applying npm's own `os`, `cpu` and `libc` fields: Vite's lockfile lists 53 platform binaries, and a Linux arm64 build needs 2 of them.
- **Downloading:** only from `registry.npmjs.org`, with no redirects.
- **Checking:** each download is hashed as it streams to disk and **published into the cache only if the SHA-512 matches**. The rename is atomic, so the cache never holds an unverified file, and two builds fetching the same package can't see half of one.
- **Reusing:** files are named by their digest, so any app's later build reuses them. Express's 66 packages download once.
- **Rechecking:** when a cached file is copied into a build it is hashed again (`npmfetch.Verify`). A file that changed on disk is caught and forgotten, so it gets downloaded again.
- **Limits:** one tarball is capped at 64 MiB, one build's packages at 160 MiB.

Downloads run **eight at a time** under one `errgroup`, and the rules for concurrent work in `AGENTS.md` are covered:
- **Ownership:** `Fetch` owns its downloads.
- **Cancellation:** the job's context cancels them all.
- **Capacity:** eight at a time.
- **Errors:** the first failure stops the rest and is returned.
- **Shutdown:** `Fetch` returns only after every download has stopped.

The fetch runs on the API host, but nothing in a package is ever **run** there: Go only hashes bytes, just as it already does for uploads. The rule that untrusted code never runs on the control-plane host still holds.

**Trade-off.** The control plane now makes outbound requests, but to one fixed host, for paths taken from a lockfile that has already been checked. The cache grows without bound for now; a sweep belongs to the "artifact retention" work.

## Story 3: The offline install, and the release that carries packages

**Problem.** Inside the container npm must install from the files Sprout provided, the release must carry only what the app needs to *run*, and Sprout's rule against symlinks in build output must still hold.

**History.** Offline npm installs have long worked by pre-filling npm's content-addressed cache (`cacache`): when npm already has a tarball whose checksum matches the lockfile, it never asks the network. Docker image builds learned to keep runtime images small with "multi-stage builds": build with all the tools, then copy only what running needs.

**Decision in Sprout** (`nodeapp.Runner`).
- **The input:** the app and the verified tarballs are streamed into the container as one tar. It goes through a private temporary file, because packages can be far bigger than source.
- **Install:** inside the container, `npm cache add` loads the tarballs, then `npm ci --offline --ignore-scripts` installs.
- **Build and prune:** after the build, `npm prune --omit=dev` removes build-only packages. Vite and TypeScript never reach a server's release.
- **The release:** `dist/`, `package.json` and the production `node_modules`, checked with new artifact limits (12,000 entries, 15 MiB). Files inside `node_modules` are exempt from the "sensitive file name" rules, which exist to protect the person's own folder; packages sometimes ship files like `test.key`.
- **Commands (`node_modules/.bin`):** these are symlinks, which never leave a build. The release step recreates them inside its own offline, capability-free container with `npm rebuild --ignore-scripts`, which links commands without running anything.

One surprise from testing: **Docker mounts temporary filesystems `noexec` by default**, so `vite` failed with "Permission denied". The workspace is now mounted `exec`. That grants nothing new, because the build can already run any code through `node`.

**Trade-off.** A release with packages is bigger (Express is about 4 MiB) and must stay under 16 MiB, the size the database allows for an artifact.

## Story 4: A Node project that is really a website

**Problem.** The most common thing people and agents build with npm is a website: a Vite, React or Svelte project whose build produces static files. It needs Node to *build*, not to *run*.

**History.** Hosts like Netlify and Vercel made "build command + output folder" the standard way to deploy front-end projects: run the build, then serve the output folder from a CDN, with no server process at all.

**Decision in Sprout.**
- **The rule:** a Node project **without a `start` script** builds a website (`nodeapp.IsWebsite`). It is built in the same sealed container, and only what is inside `dist/` comes back.
- **The checks:** the output gets the **same rules as a plain static site**: an `index.html` at the top, web-safe names, and at most 256 files and 8 MiB.
- **Going live:** the deployment records the static runtime, so it is served straight from the stored artifact, like any static site. No process runs and nothing needs to wake up.
- **A local `dist/` is left out:** packing a Node project, in the CLI or the browser, now leaves out its own `dist/`, because Sprout rebuilds it and the server refuses an uploaded one. Before, a project someone had built locally was refused with a confusing message.

**Trade-off.** The output folder is always `dist/`, which is Vite's default. Tools that use `build/` or `out/` need their output setting changed for now.

## Request trace: `sprout deploy dev/vite-example`

1. The CLI packs the folder, leaving out `node_modules` and `dist/`, then checks it against the server's rules and uploads it.
2. The worker's build job: `nodeapp.Validate` checks the lockfile (registry URLs, checksums). There is no `start` script, so the runtime is recorded as static.
3. `npmfetch.Fetch` selects 13 of the lockfile's 64 entries for linux/arm64/glibc. It downloads only what the cache lacks, and checks and caches each tarball.
4. `writeInput` streams `app/` and `packages/*.tgz`, re-verified, into a temporary tar.
5. Sealed container (`--network none`, read-only root, no capabilities): `npm cache add`, `npm ci --offline --ignore-scripts`, `npm run build`, then a tar of `dist/`'s contents.
6. `BuildWebsite` checks the output with the static-site rules and packs it. The artifact is saved and goes live as a static site.

## Verification

```sh
cd backend
go test ./...
SPROUT_DOCKER_TEST=1 go test -p 1 ./... -count=1   # needs Docker, the pinned image and internet access
cd ../frontend && node --test tests/*.test.mjs
```

Run Docker tests with `-p 1`: the runtime tests include the startup cleanup, which removes Sprout containers it doesn't know about, including another package's test release.

Unit tests:
- **The fetcher:** a tampered tarball never enters the cache; a changed cache file is caught and forgotten; a second build downloads nothing; other platforms' binaries are skipped; npm's `!os` exclusions are applied; non-registry, plain-HTTP, query and `..` URLs are refused; the cache must be private.
- **The build job:** a project without `start` goes live as a static site without touching the runtime driver.
- **The contract and packers:** packages no longer block a deploy; a blank `start` is still refused; a Node project's `dist/` is left out while a website's folder called `dist` is kept.

Docker tests:
- **Express:** `dev/node-express-example` builds, the devDependency `is-number` is pruned, `.bin` stays out, and the app goes live and answers `/health`. Without its packages, the build reports `node_build_install_failed`.
- **Vite:** `dev/vite-example` builds offline into a site with `index.html` and a bundled script.

By hand, against the API running from this branch:
- `sprout deploy dev/node-express-example` went live in 4 s and answered with Express's own header.
- `sprout deploy dev/vite-example` went live in 5 s; the page's bundled script ran in the browser.
- A release step with `semver` recreated `node_modules/.bin/semver`, runnable by the unprivileged runtime user.

## Known gaps

- **Packages:** packages that need install scripts (native modules) don't work, and only the public npm registry is supported (no private packages or `.npmrc`).
- **Tools and layout:** no Yarn or pnpm, no workspaces (monorepos), and a website's output folder must be `dist/`.
- **Limits:** the package cache is never swept, and releases are capped at 16 MiB.
- **Framework servers:** frameworks with their own server (Next.js) need `.next/` in the release and are not supported yet.

## Explain-back checkpoint

1. Why can the build stay `--network none` even though the app needs packages from the internet?
2. What exactly stops a tampered tarball from reaching a build, and what stops a tampered *cached* file?
3. Why is it safe for the API host to download packages, when untrusted code must never run there?
4. Why does `npm prune --omit=dev` run *after* the build and not before?
5. Why does a project without a `start` script never start a container, and what checks its output?
6. Why did `vite` fail with "Permission denied", and why is mounting the workspace `exec` not a loss of safety?
