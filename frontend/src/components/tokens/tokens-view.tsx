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

  return <main className="dashboard-page compact-page dash-stack">
    <PageHeader eyebrow="Settings" title="Access tokens" description="Let the Sprout CLI, CI and coding agents deploy as you, without your password." />
    {change.isError && <p className="dash-callout dash-callout--error" role="alert">{change.error.message}</p>}
    {notice && <p className="dash-callout dash-callout--ok" role="status">{notice}</p>}

    <section className="dash-panel-card">
      <div className="dash-panel-card-head"><div><h3>Create a token</h3><p>A token can create apps and deploy them, and read your workspace. It cannot manage your team, change who has access, or create other tokens. Revoke it any time.</p></div></div>
      <form className="dash-inline-form" onSubmit={(event) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        change.mutate({ kind: "create", name: String(data.get("name")).trim(), expiresInDays: Number(data.get("days")) });
        event.currentTarget.reset();
      }}>
        <label>Name<input name="name" required maxLength={80} placeholder="Laptop, GitHub Actions…" disabled={change.isPending} /></label>
        <label>Expires<select name="days" defaultValue="90" disabled={change.isPending}>{expiryChoices.map((choice) => <option key={choice.days} value={choice.days}>{choice.label}</option>)}</select></label>
        <button className="lp-btn lp-btn--dark lp-btn--small" disabled={change.isPending}>Create token</button>
      </form>
      {secret && <div className="dash-secret">
        <label>Copy this token now — it is shown only once and cannot be retrieved later<input readOnly value={secret} onFocus={(event) => event.target.select()} /></label>
        <button className="lp-btn lp-btn--light lp-btn--small" type="button" onClick={async () => {
          try { await navigator.clipboard.writeText(secret); setNotice("Token copied."); }
          catch { setNotice("Select the token above and copy it manually."); }
        }}>Copy token</button>
        <button className="lp-btn lp-btn--light lp-btn--small" type="button" onClick={() => setSecret("")}>I have saved it</button>
      </div>}
      <p className="dash-panel-card-foot">Send it as <code>Authorization: Bearer &lt;token&gt;</code>. Treat it like a password: anyone who has it can deploy as you until it expires or you revoke it.</p>
    </section>

    <section className="dash-panel-card">
      <div className="dash-panel-card-head"><h3>Your tokens</h3><small>{tokens.data?.length === 1 ? "1 token" : `${tokens.data?.length ?? 0} tokens`}</small></div>
      {tokens.isLoading && <p className="dash-note" role="status">Loading tokens…</p>}
      {tokens.isError && <p className="dash-callout dash-callout--error" role="alert">{tokens.error.message}</p>}
      {tokens.data?.length === 0 && <p className="dash-note">No tokens yet.</p>}
      <ul className="dash-members">
        {tokens.data?.map((token, index) => <li className="dash-member" key={token.id} style={{ "--i": index } as React.CSSProperties}>
          <span className="dash-member-name"><strong>{token.name}</strong><small>{displayHint(token.hint)} · {token.expired ? "Expired" : describeExpiry(token.expiresAt)} · {describeLastUsed(token.lastUsedAt)}</small></span>
          <button className="lp-btn lp-btn--light lp-btn--small dash-danger" type="button" disabled={change.isPending} onClick={() => setRevoking(token.id)}>{token.expired ? "Remove" : "Revoke"}<span className="sr-only"> {token.name}</span></button>
          {revoking === token.id && <div className="dash-confirm" role="group" aria-label={`Confirm revoking ${token.name}`}>
            <p>{token.expired ? "Remove" : "Revoke"} “{token.name}”? Anything using it loses access immediately.</p>
            <button className="lp-btn lp-btn--light lp-btn--small" type="button" disabled={change.isPending} onClick={() => setRevoking(null)}>Cancel</button>
            <button className="lp-btn lp-btn--dark lp-btn--small" type="button" disabled={change.isPending} onClick={() => change.mutate({ kind: "revoke", id: token.id })}>Confirm</button>
          </div>}
        </li>)}
      </ul>
    </section>
  </main>;
}
