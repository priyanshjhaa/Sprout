"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api/api";
import { useTokens } from "@/lib/query/hooks";
import { queryKeys } from "@/lib/query/keys";
import { useSessionAuth as useAuth } from "@/lib/query/session";
import { describeExpiry, describeLastUsed, displayHint, expiryChoices } from "@/lib/tokens";
import { PageHeader } from "@/components/dashboard/page-header";

type TokenAction = { kind: "create"; name: string; expiresInDays: number } | { kind: "revoke"; id: string };

export function TokensView() {
  const tokens = useTokens();
  const { getToken, userId } = useAuth();
  const client = useQueryClient();
  // The secret lives only in component memory: never in the query cache or storage.
  const [secret, setSecret] = useState("");
  const [notice, setNotice] = useState("");
  const [revoking, setRevoking] = useState<string | null>(null);

  const change = useMutation({
    gcTime: 0,
    mutationFn: async (action: TokenAction) => {
      const session = await getToken();
      if (action.kind === "create") {
        const created = await api.createToken({ name: action.name, expiresInDays: action.expiresInDays }, session);
        setSecret(created.token);
        return;
      }
      await api.revokeToken(action.id, session);
    },
    onMutate: () => { setNotice(""); setSecret(""); },
    onSuccess: async (_, action) => {
      setRevoking(null);
      setNotice(action.kind === "create" ? "Token created." : "Token revoked. It stopped working immediately.");
      await client.invalidateQueries({ queryKey: queryKeys.tokens(userId) });
    },
  });

  return <main className="dashboard-page compact-page">
    <PageHeader eyebrow="Settings" title="Access tokens" description="Let the Sprout CLI, CI and coding agents deploy as you, without your password." />
    {change.isError && <p role="alert">{change.error.message}</p>}
    {notice && <p role="status">{notice}</p>}

    <section className="team-invite-panel">
      <h2>Create a token</h2>
      <p>A token can create apps and deploy them, and read your workspace. It cannot manage your team, change who has access, or create other tokens. Revoke it any time.</p>
      <form className="team-invite-form" onSubmit={(event) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        change.mutate({ kind: "create", name: String(data.get("name")).trim(), expiresInDays: Number(data.get("days")) });
        event.currentTarget.reset();
      }}>
        <label>Name<input name="name" required maxLength={80} placeholder="Laptop, GitHub Actions…" disabled={change.isPending} /></label>
        <label>Expires<select name="days" defaultValue="90" disabled={change.isPending}>{expiryChoices.map((choice) => <option key={choice.days} value={choice.days}>{choice.label}</option>)}</select></label>
        <button className="button button-primary" disabled={change.isPending}>Create token</button>
      </form>
      {secret && <div className="team-share-link">
        <label>Copy this token now — it is shown only once and cannot be retrieved later<input readOnly value={secret} onFocus={(event) => event.target.select()} /></label>
        <button className="button button-secondary" onClick={async () => {
          try { await navigator.clipboard.writeText(secret); setNotice("Token copied."); }
          catch { setNotice("Select the token above and copy it manually."); }
        }}>Copy token</button>
        <button className="button button-secondary" onClick={() => setSecret("")}>I have saved it</button>
      </div>}
      <small>Send it as <code>Authorization: Bearer &lt;token&gt;</code>. Treat it like a password: anyone who has it can deploy as you until it expires or you revoke it.</small>
    </section>

    <section className="members-panel">
      <header><span>Your tokens</span><small>{tokens.data?.length === 1 ? "1 token" : `${tokens.data?.length ?? 0} tokens`}</small></header>
      {tokens.isLoading && <p className="team-panel-note" role="status">Loading tokens…</p>}
      {tokens.isError && <p className="team-panel-note" role="alert">{tokens.error.message}</p>}
      {tokens.data?.length === 0 && <p className="team-panel-note">No tokens yet.</p>}
      {tokens.data?.map((token) => <div className="member-row team-member-row" key={token.id}>
        <div><strong>{token.name}</strong><small>{displayHint(token.hint)} · {token.expired ? "Expired" : describeExpiry(token.expiresAt)} · {describeLastUsed(token.lastUsedAt)}</small></div>
        <button className="button button-secondary" disabled={change.isPending} onClick={() => setRevoking(token.id)}>{token.expired ? "Remove" : "Revoke"}<span className="sr-only"> {token.name}</span></button>
        {revoking === token.id && <div className="team-remove-confirm" role="group" aria-label={`Confirm revoking ${token.name}`}>
          <p>{token.expired ? "Remove" : "Revoke"} “{token.name}”? Anything using it loses access immediately.</p>
          <button className="button button-secondary" disabled={change.isPending} onClick={() => setRevoking(null)}>Cancel</button>
          <button className="button button-primary" disabled={change.isPending} onClick={() => change.mutate({ kind: "revoke", id: token.id })}>Confirm</button>
        </div>}
      </div>)}
    </section>
  </main>;
}
