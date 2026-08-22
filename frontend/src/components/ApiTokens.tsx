import Icon from "../Icon";
import { useEffect, useState } from "react";
import { tokenApi } from "../api";
import type { ApiToken } from "../types";
import { useLang } from "../LangContext";
import { useDialog } from "../DialogContext";

/**
 * Personal access tokens for PicGo / Typora / curl. The plaintext is shown
 * exactly once, on creation — the server only ever stores its SHA-256.
 */
export default function ApiTokens() {
  const dialog = useDialog();
  const { t, locale } = useLang();
  const [tokens, setTokens] = useState<ApiToken[]>([]);
  const [name, setName] = useState("");
  const [days, setDays] = useState(0);
  const [fresh, setFresh] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  async function load() {
    setTokens(await tokenApi.list());
  }
  useEffect(() => {
    load().catch(() => {});
  }, []);

  async function create() {
    if (!name.trim()) return;
    setBusy(true);
    setErr(null);
    try {
      const res = await tokenApi.create(name.trim(), days);
      setFresh(res.plain);
      setName("");
      await load();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  async function remove(tok: ApiToken) {
    const ok = await dialog.confirm({
      title: t.apiTokens.deleteConfirm(tok.name),
      danger: true,
      confirmLabel: t.common.delete,
    });
    if (!ok) return;
    setBusy(true);
    try {
      await tokenApi.remove(tok.id);
      await load();
    } finally {
      setBusy(false);
    }
  }

  return (
    <div>
      {fresh && (
        <div className="mb-3 rounded-xl border border-brand-500/30 bg-brand-950/20 p-3">
          <div className="text-[11px] text-brand-200 mb-2">
            <Icon name="triangle-exclamation" className="mr-1"  />
            {t.apiTokens.shownOnce}
          </div>
          <div className="flex items-center gap-1.5">
            <input
              readOnly
              value={fresh}
              onFocus={(e) => e.currentTarget.select()}
              className="flex-1 min-w-0 rounded-md bg-neutral-950 border border-neutral-800 px-2 py-1.5 text-[11px] font-mono text-neutral-200 outline-none"
            />
            <button
              onClick={async () => {
                try {
                  await navigator.clipboard.writeText(fresh);
                  setCopied(true);
                  setTimeout(() => setCopied(false), 1500);
                } catch {}
              }}
              className="shrink-0 rounded-md bg-brand-600 px-2.5 py-1.5 text-[10px] text-brand-ink hover:bg-brand-500 transition"
            >
              <Icon name={copied ? "check" : "copy"} className={`mr-1`}  />
              {copied ? t.common.copied : t.common.copy}
            </button>
            <button
              onClick={() => setFresh(null)}
              className="shrink-0 rounded-md bg-neutral-800 px-2.5 py-1.5 text-[10px] text-neutral-400 hover:bg-neutral-700 transition"
            >
              {t.apiTokens.gotIt}
            </button>
          </div>
        </div>
      )}

      {err && <div className="mb-3 text-xs text-red-400">{err}</div>}

      {tokens.length > 0 && (
        <div className="space-y-1.5 mb-3">
          {tokens.map((tok) => (
            <div
              key={tok.id}
              className="flex items-center gap-3 rounded-lg border border-neutral-800 bg-neutral-950/40 px-3 py-2"
            >
              <div className="flex-1 min-w-0">
                <div className="text-xs text-neutral-200">
                  {tok.name}
                  {tok.revoked && <span className="ml-1.5 text-[10px] text-red-400">{t.apiTokens.revoked}</span>}
                </div>
                <div className="text-[10px] text-neutral-600 font-mono">
                  {tok.prefix}••••••
                  {tok.last_used_at
                    ? ` · ${t.common.lastUsed(new Date(tok.last_used_at).toLocaleDateString(locale))}`
                    : ` · ${t.apiTokens.neverUsed}`}
                  {tok.expires_at && ` · ${t.apiTokens.expiresAt(new Date(tok.expires_at).toLocaleDateString(locale))}`}
                </div>
              </div>
              <button
                onClick={() => remove(tok)}
                disabled={busy}
                className="shrink-0 text-[10px] text-red-400 hover:underline"
              >
                {t.common.delete}
              </button>
            </div>
          ))}
        </div>
      )}

      <div className="flex items-center gap-2">
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && create()}
          placeholder={t.apiTokens.namePlaceholder}
          className="flex-1 rounded-lg bg-neutral-900 border border-neutral-800 h-8 px-2.5 text-xs outline-none focus:border-brand-500 placeholder-faint"
        />
        <select
          value={days}
          onChange={(e) => setDays(Number(e.target.value))}
          className="h-8 rounded-lg bg-neutral-900 border border-neutral-800 px-2 text-xs outline-none focus:border-brand-500"
        >
          <option value={0}>{t.apiTokens.expiry.never}</option>
          <option value={30}>{t.apiTokens.expiry.days30}</option>
          <option value={90}>{t.apiTokens.expiry.days90}</option>
          <option value={365}>{t.apiTokens.expiry.year1}</option>
        </select>
        <button
          onClick={create}
          disabled={busy || !name.trim()}
          className="inline-flex h-8 items-center justify-center rounded-lg bg-brand-600 px-3 text-xs font-medium text-brand-ink hover:bg-brand-500 disabled:bg-neutral-800 disabled:text-neutral-600 transition whitespace-nowrap"
        >
          {t.apiTokens.generate}
        </button>
      </div>

      <p className="mt-2 text-[10px] text-neutral-600">
        {t.apiTokens.scopeNote}
      </p>
    </div>
  );
}
