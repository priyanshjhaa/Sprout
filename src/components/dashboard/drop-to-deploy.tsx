"use client";

import { useSessionAuth as useAuth } from "@/lib/query/session";
import { useQueryClient } from "@tanstack/react-query";
import { FileArchive, Sprout } from "lucide-react";
import { useRouter } from "next/navigation";
import { FormEvent, useEffect, useRef, useState } from "react";
import { builds } from "@/lib/api/simulations";
import { queryKeys } from "@/lib/query/keys";
import { useCreateApplication } from "@/lib/query/hooks";
import { useCapabilities } from "@/lib/query/simulations";
import { slugify } from "@/lib/slug";

const MAX_ARCHIVE_BYTES = 16 * 1024 * 1024;

const hasFiles = (event: DragEvent) => Array.from(event.dataTransfer?.types ?? []).includes("Files");

// Drop an app's .tar anywhere on the page: name it, and Sprout creates the app
// and starts its build. Only offered when the API reports local builds are on.
export function DropToDeploy({ workspaceSlug }: { workspaceSlug: string }) {
  const enabled = useCapabilities().data?.localBuilds === true;
  const { getToken, userId } = useAuth();
  const client = useQueryClient();
  const router = useRouter();
  const create = useCreateApplication(workspaceSlug);
  const dialog = useRef<HTMLDialogElement>(null);
  const [dragging, setDragging] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [name, setName] = useState("");
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!enabled) return;
    let depth = 0;
    const enter = (event: DragEvent) => { if (hasFiles(event)) { depth += 1; setDragging(true); } };
    const leave = (event: DragEvent) => { if (hasFiles(event) && (depth = Math.max(0, depth - 1)) === 0) setDragging(false); };
    const over = (event: DragEvent) => { if (hasFiles(event)) event.preventDefault(); };
    const drop = (event: DragEvent) => {
      if (!hasFiles(event)) return;
      event.preventDefault();
      depth = 0;
      setDragging(false);
      const dropped = event.dataTransfer?.files[0];
      if (!dropped) return;
      const stem = dropped.name.replace(/\.tar$/i, "").replace(/[-_]+/g, " ").trim();
      setFile(dropped); setName(stem); setProblem(!/\.tar$/i.test(dropped.name) ? "Sprout takes a plain, uncompressed .tar archive." : dropped.size > MAX_ARCHIVE_BYTES ? "The archive is larger than 16 MiB." : null);
      dialog.current?.showModal();
    };
    window.addEventListener("dragenter", enter); window.addEventListener("dragleave", leave);
    window.addEventListener("dragover", over); window.addEventListener("drop", drop);
    return () => {
      window.removeEventListener("dragenter", enter); window.removeEventListener("dragleave", leave);
      window.removeEventListener("dragover", over); window.removeEventListener("drop", drop);
    };
  }, [enabled]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!file || problem) return;
    setBusy(true);
    try {
      const app = await create.mutateAsync({ name: name.trim(), slug: slugify(name) });
      const job = await builds.upload(workspaceSlug, app.id, file, await getToken());
      await client.invalidateQueries({ queryKey: queryKeys.builds(userId, workspaceSlug, app.id) });
      dialog.current?.close();
      router.push(`/workspace/${workspaceSlug}/apps/${app.id}/deployments/${job.id}?kind=build`);
    } catch (error) {
      setProblem(error instanceof Error ? error.message : "Could not start the build.");
    } finally {
      setBusy(false);
    }
  }

  if (!enabled) return null;
  return (
    <>
      <div className={`sp-drop-veil${dragging ? " on" : ""}`} aria-hidden="true">
        <div><FileArchive size={30} /><strong>Drop to plant it</strong><span>Release your .tar to create an app and start its build.</span></div>
      </div>
      <dialog ref={dialog} className="sp-palette sp-drop-dialog" aria-label="Deploy the dropped archive" onClose={() => setBusy(false)}>
        <form onSubmit={submit}>
          <p className="sp-drop-file"><FileArchive size={16} /> <code>{file?.name}</code></p>
          <label>Name this app<input value={name} onChange={(event) => setName(event.target.value)} required maxLength={120} disabled={busy} autoFocus /></label>
          <small>URL name: <code>{slugify(name) || "set from the name"}</code></small>
          {problem && <p className="deploy-error" role="alert">{problem}</p>}
          <div className="sp-drop-actions">
            <button type="button" className="sp-button sp-button-outline" onClick={() => dialog.current?.close()} disabled={busy}>Cancel</button>
            <button className="sp-button sp-button-solid" disabled={busy || !name.trim() || !slugify(name) || !!problem}><Sprout size={15} /> {busy ? "Planting…" : "Create and build"}</button>
          </div>
        </form>
      </dialog>
    </>
  );
}
