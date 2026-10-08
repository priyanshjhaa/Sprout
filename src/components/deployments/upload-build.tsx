"use client";

import { Upload } from "lucide-react";
import { useRouter } from "next/navigation";
import { FormEvent, useState } from "react";
import { useBuildActions } from "@/lib/query/simulations";

const MAX_ARCHIVE_BYTES = 16 * 1024 * 1024;

// Upload a plain .tar of the application and open its build. Only shown when
// the API reports that local builds are enabled.
export function UploadBuild({ workspaceSlug, appId, disabled = false }: { workspaceSlug: string; appId: string; disabled?: boolean }) {
  const action = useBuildActions(workspaceSlug, appId);
  const router = useRouter();
  const [archive, setArchive] = useState<File | null>(null);
  const [problem, setProblem] = useState<string | null>(null);

  function submit(event: FormEvent) {
    event.preventDefault();
    if (!archive) return;
    if (archive.size > MAX_ARCHIVE_BYTES) {
      setProblem("The archive is larger than 16 MiB.");
      return;
    }
    setProblem(null);
    action.mutate({ upload: archive }, {
      onSuccess: (job) => router.push(`/workspace/${workspaceSlug}/apps/${appId}/deployments/${job.id}?kind=build`),
    });
  }

  return (
    <form className="upload-build" onSubmit={submit}>
      <label>
        <span className="sr-only">Application archive (.tar)</span>
        <input type="file" accept=".tar,application/x-tar" disabled={disabled || action.isPending}
          onChange={(event) => { setArchive(event.target.files?.[0] ?? null); setProblem(null); }} />
      </label>
      <button className="button button-primary" type="submit" disabled={disabled || !archive || action.isPending}>
        <Upload size={14} /> {action.isPending ? "Uploading…" : "Upload & build"}
      </button>
      {(problem || action.isError) && <p className="deploy-error" role="alert">{problem ?? action.error?.message}</p>}
    </form>
  );
}
