"use client";

import { FileArchive, FolderUp, Rocket } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { SourcePicker } from "@/components/deploy/source-picker";
import { problemMessage, skippedNote, sourceBlob, summarise, type Chosen } from "@/lib/chosen-source";
import { useBuildActions } from "@/lib/query/simulations";

// Give Sprout an app to deploy: drop or choose its folder (Sprout packs it), or
// pick a ready-made .tar. Only shown when the API reports that local builds are on.
export function UploadBuild({ workspaceSlug, appId, disabled = false, compact = false }: { workspaceSlug: string; appId: string; disabled?: boolean; compact?: boolean }) {
  const action = useBuildActions(workspaceSlug, appId);
  const router = useRouter();
  const [chosen, setChosen] = useState<Chosen | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const [reading, setReading] = useState(false);
  const [packing, setPacking] = useState(false);
  const busy = packing || action.isPending;

  async function deploy() {
    if (!chosen) return;
    setProblem(null);
    setPacking(true);
    try {
      const blob = await sourceBlob(chosen);
      action.mutate({ upload: blob }, {
        onSuccess: (job) => router.push(`/workspace/${workspaceSlug}/apps/${appId}/deployments/${job.id}?kind=build`),
      });
    } catch (error) {
      setProblem(problemMessage(error));
    } finally {
      setPacking(false);
    }
  }

  const note = chosen ? skippedNote(chosen) : null;
  return (
    <div className="upload-build">
      {chosen ? (
        <div className="sp-chosen" role="status">
          <span className="sp-chosen-line">{chosen.kind === "folder" ? <FolderUp size={16} /> : <FileArchive size={16} />} <strong>{summarise(chosen)}</strong></span>
          {note && <small>{note}</small>}
          <div className="sp-chosen-actions">
            <button className="sp-button sp-button-solid" type="button" disabled={disabled || busy} onClick={() => void deploy()}>
              <Rocket size={14} /> {busy ? "Deploying…" : "Deploy it"}
            </button>
            <button className="sp-dropzone-link" type="button" disabled={busy} onClick={() => { setChosen(null); setProblem(null); }}>Choose something else</button>
          </div>
        </div>
      ) : reading ? (
        <p className="simulation-note" role="status">Reading your folder…</p>
      ) : (
        <SourcePicker compact={compact} disabled={disabled}
          onReading={() => { setReading(true); setProblem(null); }}
          onChosen={(next) => { setReading(false); setChosen(next); }}
          onProblem={(message) => { setReading(false); setProblem(message); }} />
      )}
      {(problem || action.isError) && <p className="deploy-error" role="alert">{problem ?? action.error?.message}</p>}
    </div>
  );
}
