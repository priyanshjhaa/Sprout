# Deployment strategy

This records the product decisions that shape how Sprout turns code into a running application. Implementation detail lives in each milestone's learning note. Revisit a decision here, with the reason, when evidence changes it.

## 1. What Sprout hosts (v1)

Sprout hosts **small applications, publicly**.

- One web process listening on `PORT` and answering `/health`, or a static site.
- Low traffic: internal tools, demos, prototypes, agent-built apps.
- **Public URL by default.** Anyone with the link can open a live app.
- Apps sleep when idle and wake on the first request, so many small apps stay cheap.
- Workspace roles (owner, editor, viewer) and per-app grants control who can **deploy, change, pause or delete** an app inside Sprout. They do not control who can visit its URL.

Not in v1: multi-service apps, background queues, persistent disks, GPUs. **Private apps** (a login gate in front of an app's URL) are a planned option after v1, not the default.

## 2. What users bring

Every deployment starts from one universal input: **a source archive** sent to one deploy API (`POST /api/v1/workspaces/{workspace}/applications/{app}/deployments`).

Front doors, in order:

1. **CLI: `sprout deploy`** packs the current folder and uploads it.
2. **MCP `deploy` tool:** coding agents deploy through the same API. One tool accepts either
   - a **local folder**, packed and uploaded like the CLI, or
   - a **GitHub repository and commit**. Sprout downloads that commit's tarball. Public repositories come first; private repositories need a GitHub App later.
3. **Dashboard upload:** a secondary door, useful for testing.

A coding agent is a *client* of the deploy API. Sprout does not generate code and does not accept chat transcripts as input.

## 3. Which apps can build (v1), in order

1. **Static sites:** a build that produces `index.html` and has no server. Served directly, with no running process.
2. **Node.js web apps:** the current contract (`node-build-contract.md`), extended with real `npm` installs through a controlled registry proxy.
3. **Python web apps:** through buildpacks (Cloud Native Buildpacks or Railpack, chosen by a short spike).

Sprout, not the application, always chooses the build image and the runtime settings. User-supplied Dockerfiles are not accepted.

## 4. Where applications run

Sprout is the **control plane**: API, permissions, lifecycle, routing. Applications run behind a `runtime.Driver` boundary (deploy, start, stop, status, destroy), so the hosting choice can change without changing the product.

- **First driver: local Docker**, for development and learning on one machine.
- **Cloud driver: decided by a hands-on trial** once the local runtime works. The same example app is deployed to Fly Machines and to Google Cloud Run and compared on cost, cold start, setup effort and isolation. The default, if no strong preference emerges, is Fly.

| | Own servers | Fly Machines | Cloud Run |
|---|---|---|---|
| Isolation of untrusted code | Our job (gVisor helps) | Microvm per app | gVisor or microVM |
| Sleep when idle | We build it | Built in | Built in |
| Cost of idle apps | Lowest | Low; stopped disks still bill | Free tier, then low |
| Operations work | Highest | Lowest | Medium |

**Fixed rule, whatever the host:** untrusted code (builds and applications) never runs on the machine that holds Sprout's database and secrets.

## 5. Roadmap

1. Align docs (this document).
2. **Local runtime:** run a built app in a locked-down container, health-check it, serve it publicly at `{app}.{workspace}.localhost`, and make pause, resume, archive and idle sleep real.
3. Static sites.
4. CLI `sprout deploy` (with CLI authentication designed first), then the MCP `deploy` tool (folder and public GitHub modes).
5. `npm` installs through a controlled registry proxy.
6. Python through buildpacks.
7. Hosting trial, then the cloud driver.
8. Later: private apps, private GitHub repositories, Postgres, encrypted secrets, artifact retention, permanent delete.

## Before anything leaves localhost

Artifact retention, a dependency egress policy, CLI token security, abuse limits for public URLs (rate limits, size caps, takedown), and a review of the build and runtime isolation boundary.
