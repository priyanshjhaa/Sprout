"use client";

import { Boxes, CornerDownLeft, LayoutDashboard, Rocket, Search, Users } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { useApplications } from "@/lib/query/hooks";

interface Command { id: string; label: string; hint: string; href: string; icon: typeof Rocket }

// ⌘K / Ctrl+K: jump anywhere in the workspace without leaving the keyboard.
export function CommandPalette({ workspaceSlug }: { workspaceSlug: string }) {
  const router = useRouter();
  const dialog = useRef<HTMLDialogElement>(null);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const { data: apps } = useApplications(workspaceSlug);
  const base = `/workspace/${workspaceSlug}`;

  const commands = useMemo<Command[]>(() => [
    { id: "deploy", label: "Deploy an app", hint: "Action", href: `${base}/deploy`, icon: Rocket },
    { id: "overview", label: "Overview", hint: "Go to", href: base, icon: LayoutDashboard },
    { id: "apps", label: "All apps", hint: "Go to", href: `${base}/apps`, icon: Boxes },
    { id: "team", label: "Team", hint: "Go to", href: `${base}/team`, icon: Users },
    ...(apps ?? []).map((app) => ({ id: app.id, label: app.name, hint: app.status === "running" ? "Live app" : "App", href: `${base}/apps/${app.id}`, icon: Boxes })),
  ], [apps, base]);
  const results = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return needle ? commands.filter((command) => command.label.toLowerCase().includes(needle)) : commands;
  }, [commands, query]);

  function open() { setQuery(""); setActive(0); dialog.current?.showModal(); }
  function choose(command: Command | undefined) {
    if (!command) return;
    dialog.current?.close();
    router.push(command.href);
  }

  useEffect(() => {
    const onKey = (event: globalThis.KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        if (dialog.current?.open) dialog.current.close(); else open();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  function onKeyDown(event: KeyboardEvent) {
    if (event.key === "ArrowDown") { event.preventDefault(); setActive((index) => Math.min(index + 1, results.length - 1)); }
    else if (event.key === "ArrowUp") { event.preventDefault(); setActive((index) => Math.max(index - 1, 0)); }
    else if (event.key === "Enter") { event.preventDefault(); choose(results[active]); }
  }

  return (
    <>
      <button type="button" className="sp-search-button" onClick={open} aria-label="Search and jump (Command K)">
        <Search size={14} /><span>Jump to…</span><kbd>⌘K</kbd>
      </button>
      <dialog ref={dialog} className="sp-palette" aria-label="Command palette"
        onClick={(event) => { if (event.target === dialog.current) dialog.current?.close(); }}>
        <div className="sp-palette-input">
          <Search size={16} aria-hidden="true" />
          <input value={query} autoFocus role="combobox" aria-expanded="true" aria-controls="sp-palette-list" aria-activedescendant={results[active] ? `sp-cmd-${results[active].id}` : undefined}
            placeholder="Jump to an app or action" aria-label="Search commands"
            onChange={(event) => { setQuery(event.target.value); setActive(0); }} onKeyDown={onKeyDown} />
          <kbd>esc</kbd>
        </div>
        <ul id="sp-palette-list" role="listbox" className="sp-palette-list">
          {results.length === 0 && <li className="sp-palette-none">Nothing matches “{query}”.</li>}
          {results.map((command, index) => (
            <li key={command.id} id={`sp-cmd-${command.id}`} role="option" aria-selected={index === active}
              className={index === active ? "active" : ""} onMouseMove={() => setActive(index)} onClick={() => choose(command)}>
              <command.icon size={16} aria-hidden="true" /><span>{command.label}</span><small>{command.hint}</small>
              {index === active && <CornerDownLeft size={13} aria-hidden="true" />}
            </li>
          ))}
        </ul>
      </dialog>
    </>
  );
}
