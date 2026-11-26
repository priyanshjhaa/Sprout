# Python apps: hashed wheels in the sealed build

Sprout could build static sites and Node apps. This milestone adds **Python web apps**: Flask, FastAPI and anything else that runs as one web process. The roadmap said "through buildpacks". A short spike showed buildpacks would break the rule that builds are sealed, so Python uses the same design as npm instead: Sprout downloads and checks the packages, and the build installs them with no network.

Commits:
- `refactor: share the verified package cache between registries`
- `feat: fetch the hashed wheels a requirements.txt names into the verified cache`
- `refactor: move the sealed build container into its own package`
- `feat: build Python apps in the sealed container and run them`
- `feat: tell every front door that Python apps work`
- `fix: recognise a woken release's prepared files by its IDs`
- these docs

## The mental model

Think of the build as a locked kitchen with no phone and no door. The app's code is the cook, and nobody trusts the cook. Python apps need ingredients from the store (packages from PyPI). The app hands over a shopping list with a fingerprint for every item (`requirements.txt` with hashes). Sprout, the trusted part, does the shopping, checks every fingerprint and passes the bag through a slot. Only ready-made ingredients are accepted (**wheels**), never kits that run their own assembly script.

```
requirements.txt ──► Sprout (trusted Go) ──► pypi.org (which files does flask 3.1.3 have?)
  name==version         │                  ──► files.pythonhosted.org (the one wheel that fits)
  --hash=sha256:…       │  checks SHA-256 against requirements.txt
                        ▼
                  package cache ({sha256}-{wheel name}) ──► sealed build container   --network none
                                                            pip install --no-index --only-binary :all:
                                                                        --require-hashes --target site-packages
                                                            compile every .py (syntax check)
                                                            ──► release: the app + site-packages/
                                                                started by the Procfile's web: command
```

What a Python app needs:

```text
my-api/
  app.py
  requirements.txt   # uv pip compile requirements.in --generate-hashes -o requirements.txt
  Procfile           # web: gunicorn app:app --bind 0.0.0.0:$PORT
```

The example is `backend/dev/python-flask-example`.

## Compared with NestJS and Django

- **NestJS (npm):** `npm ci` from a lockfile is the closest match. A Node project's lockfile records each package's URL and checksum; Python's hashed `requirements.txt` records only versions and checksums, which changes how Sprout finds the files (story 3).
- **Django:** a Django app is usually deployed by `pip install -r requirements.txt` in a Docker image, or by a Heroku buildpack, then run by gunicorn with a Procfile. Sprout keeps the Procfile and gunicorn. What it replaces is the networked install: there is no `pip install` that can reach the internet.

## Story 1: Why not buildpacks

**Problem.** A Python app needs an interpreter and its packages. Someone has to put both into the build, and Sprout's rule is that the build never reaches the network.

**History.**
- **2011:** Heroku introduced buildpacks: scripts that detect a project's language, download the right runtime, install its packages and produce something runnable. Pushing code "just worked".
- **2018:** Cloud Native Buildpacks (CNCF) turned the idea into OCI images. Paketo and Heroku publish builders for it.
- **2025:** Railway's Railpack (the successor to Nixpacks) did the same with BuildKit.

All of them are designed for convenience on a trusted build machine. They download Python itself and run `pip install` against the internet during the build. Detection and building run the buildpack's own scripts, under settings the buildpack chooses. Paketo can run offline, but only with specially packaged buildpacks and a vendored package folder.

**Decision in Sprout.** No buildpacks. Python uses the design npm already proved:
- **A pinned image:** `python:3.13-slim-bookworm`, pinned by digest (`pythonapp.DefaultImage`).
- **Sprout fetches:** Sprout downloads and verifies the packages itself.
- **The same sealed container:** the build runs with no network, a read-only root, no capabilities and an unprivileged user, using the settings shared with the Node build (`internal/buildbox`).

The spike confirmed it: pip installed Flask and Gunicorn offline from the provided wheels, and the result ran under the runtime's restrictions.

**Trade-off.** No automatic detection: an app must bring a hashed `requirements.txt` and a `Procfile`. A buildpack would also compile packages that have no ready-made wheel. Sprout refuses them (story 2).

## Story 2: Wheels, and why source packages are refused

**Problem.** Installing a Python package can run code. A **source distribution** (sdist) is installed by running its `setup.py`, or its build backend, which can do anything: read files, open connections, mine cryptocurrency. It is the same danger as npm's install scripts.

**History.**
- **2000:** Python's `distutils` made `setup.py install` the standard, so every install ran the package's own code.
- **2004:** setuptools added "eggs", an early binary format.
- **2012:** PEP 427 defined the **wheel**: a zip of files that installing simply copies, with tags in its name saying which Python and system it fits (`markupsafe-3.0.4-cp313-cp313-manylinux_2_17_aarch64.whl`).
- **2016:** PEP 513 added **manylinux**, so one compiled wheel works on most Linux systems. PEP 600 later generalised it as `manylinux_X_Y`, "needs glibc X.Y or newer".
- **The stable ABI** (`abi3`, PEP 384): a wheel built for Python 3.x keeps working on every later 3.y.

Today nearly every popular package ships wheels, including compiled ones such as pydantic-core, cryptography and numpy.

**Decision in Sprout.**
- **Wheels only:** pip runs with `--only-binary :all:`, so it never builds or runs a source package.
- **Tag matching:** `pypifetch` reads each wheel's tags (`Platform.accepts`) and keeps only those the container can install:
  - **Python:** `py3`, `cp313`, or `abi3` built for 3.2 to 3.13.
  - **System:** `any`, or a manylinux wheel for this CPU that needs at most glibc 2.36, the version in Bookworm.
  - **Never:** macOS, Windows, Alpine (musllinux), free-threaded `cp313t` and unportable `linux_*` wheels.
- **No source build:** a package with no fitting wheel fails with `python_package_no_wheel`, before the build starts.

**Trade-off.** A package that only exists as source (for example `docopt 0.6.2`) can't be used. Usually a newer version, or an alternative package, ships wheels.

## Story 3: Hashes without addresses

**Problem.** npm's lockfile says where each package is (`resolved`) and what it must hash to (`integrity`). A hashed `requirements.txt` says only `flask==3.1.3` and the SHA-256 of every file the author accepts, usually one per platform. It never says where the files are.

**History.** pip added **hash-checking mode** in version 8 (2016): once any requirement has a `--hash`, every requirement must have one, and pip refuses any file whose hash isn't listed. Tools such as `pip-compile` and `uv` write the hashes for every platform. Meanwhile **dependency confusion** attacks (2021) showed what an extra package index can do: a build asked for a private package name and received a public package of the same name.

**Decision in Sprout** (`backend/internal/pypifetch`).
- **The rules:**
  - Every line is `name==version` plus `--hash=sha256:…`.
  - Lines that send pip elsewhere are refused: `-r`, `-e`, `--index-url`, `--extra-index-url`, URLs and `name @ url`.
  - A line with a marker (`; python_version < "3.11"`) is allowed. If no wheel fits, Sprout leaves the decision to pip.
- **Finding the file:** Sprout asks PyPI's JSON API, `https://pypi.org/pypi/{name}/{version}/json`, which files the version has. It keeps the wheels whose SHA-256 is listed and whose tags fit, and downloads one from `files.pythonhosted.org`. A file at any other address is refused, and redirects are never followed.
- **Checking:** the download is hashed as it streams to disk and only published to the cache if it matches. Inside the container pip checks every hash again (`--require-hashes`), and with `--no-index` it has nowhere else to look.
- **The cache name** is `{sha256}-{wheel filename}`:
  - **The hash** makes the file trustworthy and shareable, exactly as with npm.
  - **The filename** matters because pip reads a wheel's tags from its name.
  - **No network on rebuild:** a later build finds a requirement's wheel by its hash without asking PyPI at all. A cached Flask build made no requests and fetched in 6 ms.
- **One cache:** npm tarballs and Python wheels share one cache directory (`SPROUT_PACKAGE_CACHE_DIR`, by default `packages` beside the artifacts). Their names can't collide.

**Trade-off.** The API host now talks to two more fixed hosts. Each first build of a package asks PyPI about its version. Only PyPI is supported: no private indexes.

## Story 4: The Procfile, and a release that knows how it runs

**Problem.** Node apps say how to start in `package.json`. Python has no standard file for that, so Sprout needs one, and the runtime must start a Python release differently from a Node one.

**History.** Heroku introduced the **Procfile** (2011): one line per process type, such as `web: gunicorn app:app`. The Twelve-Factor App made "one process, bound to the port in `$PORT`" the norm. Buildpacks, Render, Railway and Fly all read Procfiles, so coding agents already know the format.

**Decision in Sprout.**
- **The Procfile:** it must have exactly one line, `web: <command>` (`pythonapp.WebCommand`). Other process types are refused rather than silently ignored.
- **Installing:** packages go into `site-packages/` with pip's `--target`, not a virtual environment, because a venv is made of symlinks and symlinks never leave a build.
- **Every release records its runtime:** `runtime.Release` gained a `Runtime` field, and the database allows `python` (migration `0008_python_runtime`). The Docker driver runs a Python release in the same pinned image that built it, with:
  - `PYTHONPATH=/app/site-packages`;
  - `PATH` including `site-packages/bin`;
  - `PYTHONDONTWRITEBYTECODE`, because the release is read-only;
  - the Procfile's command, run by `sh` inside the sandbox, just as `npm start` runs a shell command.
- **A syntax check:** the build compiles every `.py` file in memory, so a syntax error fails the build (`python_build_failed`) instead of the health check.

One surprise from testing: the release failed to start, with **"gunicorn: Permission denied"**. Sprout repacks every build output with fixed modes (0600), so console scripts like `gunicorn` arrived as plain files. Node never hit this because the release step recreates `node_modules/.bin`. The Python release step now marks `site-packages/bin/*` executable. That grants nothing new, since the app can already run any Python code.

**Trade-off.** One web process per app: no workers, no release-phase commands such as migrations. Python is fixed at 3.13.

## Story 5: When the second user earns the abstraction

**Problem.** The Python fetcher needed exactly what `npmfetch` already did: download, hash while streaming, publish atomically, re-verify on use. The Python build needed exactly the Node build's container: the security flags, bounded output, cleanup and tar packing.

**History.** The "rule of three" (popularised by Martin Fowler's *Refactoring*, 1999) says to tolerate a copy until the third use, because a premature abstraction guesses wrong. But it is a rule about convenience. For **security-critical code**, two copies are already a risk: a fix applied to one is missing from the other.

**Decision in Sprout.** Two small refactors came first, each with the existing tests passing unchanged:
- **`internal/pkgcache`:** decides when a download may be believed. `npmfetch` and `pypifetch` decide only *what* to download and give errors their own codes.
- **`internal/buildbox`:** owns the build container's whole security posture. `nodeapp` and `pythonapp` supply only the image, the script and what its exit codes mean.

**Trade-off.** Two more packages to read. In exchange, the sealed-build boundary exists exactly once.

## Story 6: Value or identity? The bug that stopped apps waking

**Problem.** After the API restarted, no app could wake, Node apps included. Visitors saw "Application unavailable".

**History.** Domain-driven design (Eric Evans, 2003) separates **value objects**, which are equal when all their fields are equal, from **entities**, which are equal when they are the same thing (the same ID), whatever their other fields say. Go's `==` on a struct is value equality. Adding a field to a struct changes what `==` means at every place it is used.

**What happened.** `Release` gained `Runtime`. To wake an app, the supervisor asks whether the driver already holds the release's files. It did so by comparing whole `Release` values. The driver's inventory comes from Docker labels, which record the IDs but not the runtime; the release being woken came from the database, with its runtime filled in. They never compared equal, so the supervisor unpacked the release again over its own read-only files, which failed.

**Decision in Sprout.** A release is an entity: `contains` now compares the application and deployment IDs. The test's fake driver now behaves like Docker and forgets the runtime. A new test fails without the fix and passes with it.

**Trade-off.** None worth keeping the bug for. The lesson is to search for `==` whenever a struct gains a field.

## Request trace: `sprout deploy dev/python-flask-example`

1. **Packing:** the CLI packs the folder, leaving out `__pycache__`, `venv`, `.venv` and secrets. It recognises a Python app by its `requirements.txt` and runs the server's own `pythonapp.Validate` locally, then uploads.
2. **Validation:** the worker's build job finds no `package.json` but a `requirements.txt`, and runs `pythonapp.Validate` (pinned and hashed requirements, one `web:` line, no venv or `site-packages`). The runtime is recorded as `python`.
3. **Fetching:** `pypifetch.Fetch` takes each requirement from the cache by hash, or asks PyPI and downloads a fitting wheel. Eight downloads run at a time, under one errgroup.
4. **The input:** `writeInput` streams `app/` and `wheels/{filename}`, each wheel re-verified, into a temporary tar.
5. **The sealed container** (`buildbox`) runs `pip install --no-index --only-binary :all: --require-hashes --no-compile --target site-packages`, then the syntax check, then a tar of the result.
6. **Output checks:** `BuildFiles` checks the output with the artifact limits (files under `site-packages/` are exempt from the sensitive-name rule, because packages like `certifi` ship `cacert.pem`), requires the `Procfile`, and packs it.
7. **Going live:** the driver unpacks the release in the Python image, marks the console scripts executable and makes everything read-only. It then starts the Procfile's command as user 65532 with no capabilities. `/health` answers, and the deployment goes live.

## Verification

```sh
cd backend
go test ./...
SPROUT_DOCKER_TEST=1 go test -p 1 ./... -count=1   # needs Docker, both pinned images and internet access
cd ../frontend && node --test tests/*.test.mjs && npx playwright test e2e/landing.spec.ts
```

Unit tests:
- **The fetcher, against a fake PyPI:**
  - It picks the Linux wheel over the sdist and the macOS wheel.
  - A cached build makes no requests.
  - A tampered wheel never enters the cache.
  - Files from other hosts are refused.
  - A source-only package is explained, and a missing version gets its own code.
  - A marker line without a wheel is left to pip.
  - The wheel tag table covers Python, ABI, glibc, architecture, musllinux, free-threading and build tags.
  - `requirements.txt` parsing refuses every way to send pip elsewhere.
- **The contract:** every rule names its own code; the example meets it.
- **The build job:** a Python project goes live with the Python runtime and tells the driver so; each broken rule is reported before the builder runs.
- **The runtime:** the Python run arguments get the same security flags as Node's; unknown runtimes and unpinned images are refused; a release woken from the database reuses its prepared files.
- **Packing:** `site-packages` is exempt only inside a build's output, never in an upload; the packers leave out `venv`, `site-packages` and `__pycache__`.
- **The browser:** the readiness warnings for unpinned or unhashed requirements and the Procfile.

Docker tests:
- **The Flask example:** it builds offline with markupsafe's compiled aarch64 wheel and no bytecode, then goes live through the whole build job and answers `/health`.
- **Failures:** a missing wheel fails as `python_build_install_failed`, and a syntax error as `python_build_failed`.

By hand, against the API running from this branch:
- **Flask:** `sprout deploy dev/python-flask-example` went live in 5 s.
- **FastAPI:** a FastAPI app with uvicorn and pydantic-core (14 packages) went live in 8 s.
- **Waking:** after an API restart, Flask woke in 2.6 s, FastAPI in 2.0 s and the Node Express app in 1.1 s.
- **Unhashed requirements:** a `requirements.txt` without hashes was refused before upload, and its `venv` was left out.
- **A source-only package** (`docopt`) failed with `python_package_no_wheel`.

## Known gaps

- **The size limit:** releases are capped at 16 MiB. Flask (about 4 MiB) and FastAPI fit; **Django does not** (about 43 MB and 3,800 files installed). Larger releases need the artifact store's limit raised, which belongs to artifact retention work.
- **Packages:**
  - Packages without wheels for Linux and Python 3.13 don't install.
  - Only PyPI: no private indexes.
  - Only `requirements.txt`: Poetry and uv lockfiles must be exported first with `uv export --format requirements-txt`.
- **Versions and processes:** Python is fixed at 3.13. Only a `web` process: no workers or release commands.
- **The failure message** doesn't name the package: a deployment stores only a stable failure code.
- **The package cache** is never swept.

## Explain-back checkpoint

1. Why can the Python build stay `--network none` when the app needs packages from PyPI?
2. Why does Sprout refuse source packages, and what is the npm equivalent of that rule?
3. A hashed `requirements.txt` has no URLs. How does Sprout find the right file, and why is the filename kept in the cache?
4. Why does pip still run with `--require-hashes` and `--no-index` if Sprout already checked every wheel?
5. Why are packages installed with `--target site-packages` instead of into a virtual environment?
6. Why did gunicorn fail with "Permission denied", and why didn't Node apps have the same problem?
7. Adding `Runtime` to `Release` stopped every app from waking. Which comparison broke, and why is comparing IDs the right fix?
