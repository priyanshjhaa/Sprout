"use client";

import { TriangleAlert } from "lucide-react";
import { useEffect, useState } from "react";
import type { Chosen } from "@/lib/chosen-source";
import { inspectNodeApp } from "@/lib/node-readiness";

/** What may stop the chosen folder from running on Sprout, worked out in the browser. */
export function useReadiness(chosen: Chosen | null): string[] {
  const [problems, setProblems] = useState<{ source: Chosen | null; list: string[] }>({ source: null, list: [] });
  useEffect(() => {
    if (chosen?.kind !== "folder" || chosen.folder.kind !== "node") return;
    let current = true;
    inspectNodeApp(chosen.folder.files).then((list) => { if (current) setProblems({ source: chosen, list }); }, () => undefined);
    return () => { current = false; };
  }, [chosen]);
  // Results belong to the folder they were computed for, so a new choice never shows stale warnings.
  return problems.source === chosen ? problems.list : [];
}

/** A calm heads-up before upload. It never blocks: the server has the final say. */
export function Readiness({ problems }: { problems: string[] }) {
  if (problems.length === 0) return null;
  return (
    <div className="sp-heads-up" role="status">
      <strong><TriangleAlert size={15} aria-hidden="true" /> Sprout may not be able to run this yet</strong>
      <ul>{problems.map((problem) => <li key={problem}>{problem}</li>)}</ul>
    </div>
  );
}
