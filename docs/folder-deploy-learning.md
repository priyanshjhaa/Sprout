# Deploying a folder

The deploy API takes one archive. That is the right contract for the server, which must verify exactly what it runs, but it is the wrong thing to ask of a person. Most people who have a site have a folder (from a site builder, a coding assistant, or their own editor), and many have never heard of a `.tar`. Asking for one quietly turns "deploy in seconds" into a research task. This milestone moves the packaging into the browser: the person drops a folder, and Sprout does the rest.

Commits: `feat: pack a dropped folder into a deployable archive in the browser`, `feat: deploy by dropping or choosing a folder`, plus these docs.

## The mental model

The browser is a helpful assistant; the server is the authority. The browser reads the folder, leaves out what nobody means to publish, finds the real site inside, explains problems in plain words, and packs the one format the server accepts. The server then re-checks every rule as if the browser did not exist, because it might not: the CLI, an agent, or an attacker can send an archive without ever using this page.

## Decisions

**Pack in the browser, not on the server.** The API contract does not change, so the CLI and agents keep working and the attack surface does not grow. A server that accepted folders or zip files would need a second archive parser to trust with untrusted input.

**Hand-written, minimal tar.** The packer is about sixty lines: regular files only, plain USTAR headers (a 100-byte name plus a 155-byte prefix for long paths), a fixed timestamp so output is reproducible, and no dependency. The server accepts nothing else (no extended headers, no links), so there is nothing to support beyond that.

**Leave out what should never be published, and say so.** Hidden files (anything starting with a dot: `.env`, `.git`, `.ssh`, `.DS_Store`), common key and credential files, `node_modules`, and system junk are never uploaded. This is a safety feature first: a dropped project folder cannot leak a `.env` by accident. It is also why a developer's folder "just works". The dialog lists what was left out. Folders like `node_modules` are not even opened while reading, so a large project cannot freeze the page.

**Find the real site.** The dropped folder's own name is not part of the site (`my-site/index.html` is `index.html`), and a zip's double wrapper (`site/site/index.html`) is looked through. Several loose files dropped together stay at the top. A `package.json` at the top means a Node.js app; an `index.html` means a website; neither gets a message that names what was found and hints at near misses ("Rename `Index.html` to exactly `index.html`").

**Check limits before uploading, in words.** The server's limits (256 files, 8 MiB, 2 MiB per file, 16 folders deep, plain file names for websites) are mirrored so people learn about a problem in a sentence naming the offending files, not from a failed deployment.

**Read the drop synchronously.** Browsers empty a drag's items as soon as the handler yields, so the folder is read from inside the drop event before anything is awaited. The reader walks `FileSystemEntry` objects, fetching entries in batches until a folder is exhausted.

## Saying so before it fails

A real project (a Next.js app with a database, 17 packages and no `engines` field) was dropped during testing. The upload worked, then the run failed with "check package.json, the lockfile, and the build and start scripts": true, useless, and the real reason (Sprout cannot install packages yet) was never named. Two changes came from that.

**The server names the rule.** A Node.js contract failure now carries a specific, stable code (`node_contract_engine`, `_private`, `_scripts`, `_manifest`, `_lockfile`, `_workspaces`, `_files`). The codes are fixed identifiers, never source text, and `node_contract_invalid` remains as the generic ancestor so existing checks keep working. Packages are checked first, because they are the one blocker no edit to `package.json` can fix: the app is refused with `node_build_dependencies_unsupported`, and the dashboard explains it in plain words.

**The browser says it before upload.** When a dropped folder is a Node.js app, the browser reads its `package.json` and shows a heads-up: packages first and alone ("this app uses 17 (next, react, …)"), otherwise each unmet rule in words ("package.json needs `"engines": { "node": "24.x" }`"). If the project has a finished website in `dist`, `build` or `out`, it says to drop that folder instead. It never blocks (the button becomes "Deploy anyway"), because these checks only mirror the server's, which stays the authority.

## Where it appears

The workspace overview accepts a drop anywhere on the page (create an app named after the folder, then deploy). The Deploy page and an app's Deployments tab have a visible drop zone, a **Choose a folder** button, and a small "or pick a .tar file" link. A ready-made `.tar` still works, unchanged. A zip gets a clear message ("Unzip it and drop the folder inside") because reading zips in the browser needs its own careful decoder.

## Verification

```sh
npm test && ./node_modules/.bin/tsc --noEmit && npm run lint
cd backend && go test ./internal/staticsite -run Browser
```

Tests check the packer byte by byte against the USTAR layout (checksums, sizes, padding, end blocks), cross-check it with the system `tar` (including a path long enough to need the prefix), and prove reproducibility. A fixture produced by the packer is committed, and a Go test feeds those exact bytes through the real archive extractor and static contract, so a packer change that the server would reject fails a test. Other tests cover wrapper stripping, skipped files, limits, names, near-miss hints, batched directory reads, skipped folders never being opened, and the wording. A production build confirms the new modules bundle. A realistic folder with `.env`, `.git`, `node_modules` and `.DS_Store` in it was packed by this code, deployed through the running API, and served; the secret files returned 404.

## Known gaps

No zip support yet. No progress bar for large folders (the limit is 8 MiB, so packing is instant). The browser cannot read a folder dropped from some remote or virtual file systems, in which case the "Choose a folder" button still works. The mirrored limits live in two languages and must be kept in step by hand; the Go fixture test is the safety net for the format, not for every limit.

## Explain-back checkpoint

1. Why is it safe for the browser to skip files, given the server decides what to run?
2. Why must the folder be read inside the drop handler, before any `await`?
3. Why does the packer omit directory entries and use a fixed timestamp?
4. What does the Go fixture test protect, and when would it fail?
5. Why is leaving out hidden files a safety feature before it is a convenience?
