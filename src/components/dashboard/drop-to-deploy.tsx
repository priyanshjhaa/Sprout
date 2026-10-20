"use client";

import { useSessionAuth as useAuth } from "@/lib/query/session";
import { useQueryClient } from "@tanstack/react-query";
import { FolderUp, Sprout } from "lucide-react";
import { useRouter } from "next/navigation";
import { FormEvent, useEffect, useRef, useState } from "react";
import { builds } from "@/lib/api/simulations";
import { chooseSource, problemMessage, skippedNote, sourceBlob, suggestedName, summarise, type Chosen } from "@/lib/chosen-source";
import { Readiness, useReadiness } from "@/components/deploy/readiness";
import { readDropped } from "@/lib/read-dropped";
import { queryKeys } from "@/lib/query/keys";
import { useCreateApplication } from "@/lib/query/hooks";
import { useCapabilities } from "@/lib/query/simulations";
import { slugify } from "@/lib/slug";

const hasFiles = (event: DragEvent) => Array.from(event.dataTransfer?.types ?? []).includes("Files");

// Drop a site's folder (or a ready-made .tar) anywhere on the page: name it, and
// Sprout creates the app, packs the folder and starts the deployment. Only
// offered when the API reports local builds are on.
export function DropToDeploy({ workspaceSlug }: { workspaceSlug: string }) {
  const enabled = useCapabilities().data?.localBuilds === true;
  const { getToken, userId } = useAuth();
  const client = useQueryClient();
  const router = useRouter();
  const create = useCreateApplication(workspaceSlug);
  const dialog = useRef<HTMLDialogElement>(null);
  const [dragging, setDragging] = useState(false);
  const [reading, setReading] = useState(false);
  const [chosen, setChosen] = useState<Chosen | null>(null);
  const [name, setName] = useState("");
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const warnings = useReadiness(chosen);

  useEffect(() => {
    if (!enabled) return;
    let depth = 0;
    const enter = (event: DragEvent) => { if (hasFiles(event)) { depth += 1; setDragging(true); } };
    const leave = (event: DragEvent) => { if (hasFiles(event) && (depth = Math.max(0, depth - 1)) === 0) setDragging(false); };
    const over = (event: DragEvent) => { if (hasFiles(event)) event.preventDefault(); };
    const drop = (event: DragEvent) => {
      if (!hasFiles(event) || !event.dataTransfer) return;
      event.preventDefault();
      depth = 0;
      setDragging(false);
      setChosen(null); setProblem(null); setReading(true); setName("");
      dialog.current?.showModal();
      // Read the drop now, before awaiting anything: the browser empties it once this handler yields.
      readDropped(event.dataTransfer.items)
        .then((dropped) => {
          const next = chooseSource(dropped);
          setChosen(next);
          setName(suggestedName(next));
        })
        .catch((error) => setProblem(problemMessage(error)))
        .finally(() => setReading(false));
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
    if (!chosen || problem) return;
    setBusy(true);
    try {
      // Pack first: if the folder can't be read, no empty app is left behind.
      const blob = await sourceBlob(chosen);
      const app = await create.mutateAsync({ name: name.trim(), slug: slugify(name) });
      const job = await builds.upload(workspaceSlug, app.id, blob, await getToken());
      await client.invalidateQueries({ queryKey: queryKeys.builds(userId, workspaceSlug, app.id) });
      dialog.current?.close();
      router.push(`/workspace/${workspaceSlug}/apps/${app.id}/deployments/${job.id}?kind=build`);
    } catch (error) {
      setProblem(problemMessage(error));
    } finally {
      setBusy(false);
    }
  }

  if (!enabled) return null;
  const note = chosen ? skippedNote(chosen) : null;
  return (
    <>
      <div className={`sp-drop-veil${dragging ? " on" : ""}`} aria-hidden="true">
        <div><FolderUp size={30} /><strong>Drop to plant it</strong><span>Release your site&rsquo;s folder to create an app and deploy it.</span></div>
      </div>
      <dialog ref={dialog} className="sp-palette sp-drop-dialog" aria-label="Deploy the dropped folder" onClose={() => setBusy(false)}>
        <form onSubmit={submit}>
          {reading && <p className="sp-drop-file" role="status">Reading your folder…</p>}
          {chosen && <p className="sp-drop-file"><FolderUp size={16} /> <strong>{summarise(chosen)}</strong></p>}
          {note && <small>{note}</small>}
          <Readiness problems={warnings} />
          {chosen && <>
            <label>Name this app<input value={name} onChange={(event) => setName(event.target.value)} required maxLength={120} disabled={busy} autoFocus /></label>
            <small>URL name: <code>{slugify(name) || "set from the name"}</code></small>
          </>}
          {problem && <p className="deploy-error" role="alert">{problem}</p>}
          <div className="sp-drop-actions">
            <button type="button" className="sp-button sp-button-outline" onClick={() => dialog.current?.close()} disabled={busy}>{chosen ? "Cancel" : "Close"}</button>
            {chosen && <button className="sp-button sp-button-solid" disabled={busy || !name.trim() || !slugify(name) || !!problem}><Sprout size={15} /> {busy ? "Planting…" : warnings.length > 0 ? "Deploy anyway" : "Create and deploy"}</button>}
          </div>
        </form>
      </dialog>
    </>
  );
}
