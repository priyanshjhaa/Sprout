# Source preparation: the first real filesystem boundary

This is the first small part of backend milestone 8. It prepares files for a future build; it does **not** build or execute them. Existing simulation endpoints and the dashboard are unchanged.

## Mental model

Before a worker can build an application, it needs its source files. An archive is not just a bag of harmless text: it can contain names like `../../outside`, links to other files, device entries, or enormous payloads.

Sprout now opens a private temporary directory, accepts a limited source archive, validates each entry, and writes only ordinary files inside that directory. Trusted Go code can inspect the prepared filesystem through a callback. When that callback finishes—or fails—the preparation function closes its directory handle and removes its temporary files.

In NestJS you could do the same with filesystem and archive libraries; in Django you could use Python's filesystem and tar utilities. Neither framework makes extraction safe automatically. Go makes the reader, directory handle, cancellation context, and cleanup explicit. `defer` schedules cleanup when the function returns or unwinds through a panic; it does not survive a machine crash or forced process kill.

## Trace

Local source archive → size/type check → private temporary directory → bounded archive reader → validated relative paths → root-confined writes → trusted consumer → close and cleanup → safe summary.

The `sourcecheck` developer command exercises this boundary without credentials, PostgreSQL, Docker, network access, or a public upload endpoint. It reports only file count and total bytes. Errors do not include source contents, entry names, or the archive's path.

## Deliberately narrow initial format

For this development slice, the accepted format is **plain uncompressed tar**, with relative paths and only regular files/directories. USTAR is recommended; ordinary BSD tar numeric-padding variants are also accepted after parsing. This is not a final product upload-format decision. ZIP, gzip, GNU/PAX extensions, Git cloning, symlinks, hard links, devices, and FIFOs are not supported.

- Archive input: at most 16 MiB, including padding.
- File contents: at most 8 MiB total and 2 MiB per file.
- Entries: at most 256, including directories; paths at most 240 bytes and 16 components.
- Reject absolute paths, traversal, non-canonical paths, backslashes, colons, control characters, duplicate/case-alias entries, and appended nonzero payloads.
- Files use private `0600` permissions and directories use `0700`. Archive ownership, executable/setuid bits, and timestamps are not preserved.
- Reject common sensitive paths such as `.git`, `.env` and `.env.*` (including templates for now), `.ssh`, cloud credential directories, `.npmrc`, `.netrc`, and key/certificate files.

This filename policy is **not a secret scanner**. It cannot detect credentials embedded in an ordinary source file. Do not submit secrets. Source contents never enter PostgreSQL or application logs, and this command does not persist the prepared tree after inspection.

## Ownership and limitations

`source.WithArchive` owns the temporary tree and its root handle. The caller owns the input reader. The consumer callback is trusted platform code, must honor cancellation, and must not retain open handles or launch background work that outlives the callback. No goroutines are created here.

Cancellation is checked before work and between reads. A context cannot interrupt an arbitrary blocked `io.Reader`: a future network upload adapter must enforce its own read deadline and body limit. The developer command is limited to local regular files and uses a ten-second context deadline; a stuck filesystem operation can still exceed it.

`os.Root` confines filesystem operations; it is **not a sandbox for executing application code**. The temporary directory is not a container or VM. No source code or build command runs on the host. Isolation, CPU/memory/process/network limits, controlled build instructions, crash-orphan recovery, and Docker integration must be implemented and reviewed before enabling real builds.

Disk limits apply per preparation call, not across the machine. Future integration must run under the existing bounded workers and establish a dedicated build-storage quota. SIGKILL/crashes can leave temporary directories; no broad automatic temp-directory deletion is introduced here.

## Verification

From `backend/`:

```sh
go test -race ./internal/source ./cmd/sourcecheck
go vet ./internal/source ./cmd/sourcecheck
```

Tests verify valid preparation, normalized permissions, cleanup after success/failure/panic/cancellation, malicious paths, links/devices, sensitive filenames, duplicate entries, truncation, entry/file/total/input limits, unsupported extensions, and safe command output.

To manually check a **non-sensitive** file already on your machine, create an archive containing explicit filenames (not an entire project directory, which may include ignored secrets):

```sh
tar --format=ustar -cf /tmp/sprout-example.tar -C /path/to/safe-example index.html
go run ./cmd/sourcecheck -archive /tmp/sprout-example.tar
```

Expected: JSON containing `files` and `bytes`, exit status 0, and no retained extracted tree. A rejected archive exits nonzero with an allow-listed preparation error. The input archive itself belongs to you and is not deleted.

Implementation verification: the full Go race-test suite (including local PostgreSQL integrations) and `go vet` passed. A manually created macOS USTAR archive containing one 82-byte HTML file returned `{"files":1,"bytes":82}`. A second archive containing a placeholder `.env` file was rejected with `source_sensitive_path` and a nonzero exit. No real application code was executed.

## Explain-back checkpoint

1. Why is checking file contents insufficient if an archive's filename can escape its directory?
2. Who removes partially extracted files after cancellation?
3. Why does a private folder not make it safe to execute untrusted code?

Commit boundary: `feat: add bounded source archive preparation`.

Next decision: establish the local build-isolation contract and approved input/build format before connecting real source to workers. Do not silently convert simulation jobs into real builds. The signed-in dashboard verification from milestone 7 remains a separate pending check.
