"use client";

import { FolderUp } from "lucide-react";
import { useRef, useState } from "react";
import { chooseSource, problemMessage, type Chosen } from "@/lib/chosen-source";
import { readDropped, readPicked, type Dropped } from "@/lib/read-dropped";

// The folder input is non-standard but supported by every current browser.
const folderAttributes = { webkitdirectory: "", directory: "" } as Record<string, string>;

/**
 * Where people give Sprout their site: drop the folder, choose it, or (for
 * people who have one) pick a .tar. It reports what was understood, or a
 * problem in plain words, and never uploads anything itself.
 */
export function SourcePicker({ onChosen, onProblem, onReading, disabled = false, compact = false }: {
  onChosen: (chosen: Chosen) => void;
  onProblem: (message: string) => void;
  onReading?: () => void;
  disabled?: boolean;
  compact?: boolean;
}) {
  const folderInput = useRef<HTMLInputElement>(null);
  const archiveInput = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);

  async function take(reading: Promise<Dropped>) {
    onReading?.();
    try {
      onChosen(chooseSource(await reading));
    } catch (error) {
      onProblem(problemMessage(error));
    }
  }

  const picked = (files: FileList | null) => {
    if (!files || files.length === 0) return;
    try {
      void take(Promise.resolve(readPicked(files)));
    } catch (error) {
      onProblem(problemMessage(error));
    }
  };

  const buttons = (
    <div className="dash-dropzone-actions">
      <button type="button" className="lp-btn lp-btn--dark lp-btn--small" disabled={disabled} onClick={() => folderInput.current?.click()}>
        <FolderUp size={15} /> Choose a folder
      </button>
      <button type="button" className="dash-text-button" disabled={disabled} onClick={() => archiveInput.current?.click()}>or pick a .tar file</button>
    </div>
  );

  return (
    <div
      className={`dash-dropzone${over ? " over" : ""}${compact ? " compact" : ""}`}
      onDragOver={(event) => { if (!disabled) { event.preventDefault(); setOver(true); } }}
      onDragLeave={() => setOver(false)}
      onDrop={(event) => {
        event.preventDefault();
        setOver(false);
        if (disabled) return;
        // Must be called now, before any await: the browser empties the drop once this handler yields.
        void take(readDropped(event.dataTransfer.items));
      }}
    >
      {!compact && (
        <>
          <FolderUp size={26} aria-hidden="true" />
          <strong>Drop your site&rsquo;s folder here</strong>
          <span>The folder with your <code>index.html</code> in it. Sprout packs it for you.</span>
        </>
      )}
      {buttons}
      <input ref={folderInput} type="file" className="sr-only" tabIndex={-1} aria-label="Choose your site's folder" multiple {...folderAttributes}
        onChange={(event) => { picked(event.target.files); event.target.value = ""; }} />
      <input ref={archiveInput} type="file" className="sr-only" tabIndex={-1} aria-label="Choose a .tar archive" accept=".tar,application/x-tar"
        onChange={(event) => { picked(event.target.files); event.target.value = ""; }} />
    </div>
  );
}
