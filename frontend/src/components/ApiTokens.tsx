import { useEffect, useState } from "react";
import { tokenApi } from "../api";
import type { ApiToken } from "../types";

/**
 * Personal access tokens for PicGo / Typora / curl. The plaintext is shown
 * exactly once, on creation — the server only ever stores its SHA-256.
 */
export default function ApiTokens() {
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

  async function remove(t: ApiToken) {
    if (!confirm(`删除 Token「${t.name}」？使用它的工具会立即失效。`)) return;
    setBusy(true);
    try {
      await tokenApi.remove(t.id);
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
            <i className="fa-solid fa-triangle-exclamation mr-1" />
            这个 Token 只显示这一次，请立即保存
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
              <i className={`fa-solid ${copied ? "fa-check" : "fa-copy"} mr-1`} />
              {copied ? "已复制" : "复制"}
            </button>
            <button
              onClick={() => setFresh(null)}
              className="shrink-0 rounded-md bg-neutral-800 px-2.5 py-1.5 text-[10px] text-neutral-400 hover:bg-neutral-700 transition"
            >
              知道了
            </button>
          </div>
        </div>
      )}

      {err && <div className="mb-3 text-xs text-red-400">{err}</div>}

      {tokens.length > 0 && (
        <div className="space-y-1.5 mb-3">
          {tokens.map((t) => (
            <div
              key={t.id}
              className="flex items-center gap-3 rounded-lg border border-neutral-800 bg-neutral-950/40 px-3 py-2"
            >
              <div className="flex-1 min-w-0">
                <div className="text-xs text-neutral-200">
                  {t.name}
                  {t.revoked && <span className="ml-1.5 text-[10px] text-red-400">已失效</span>}
                </div>
                <div className="text-[10px] text-neutral-600 font-mono">
                  {t.prefix}••••••
                  {t.last_used_at
                    ? ` · 最近使用 ${new Date(t.last_used_at).toLocaleDateString("zh-CN")}`
                    : " · 从未使用"}
                  {t.expires_at && ` · 到期 ${new Date(t.expires_at).toLocaleDateString("zh-CN")}`}
                </div>
              </div>
              <button
                onClick={() => remove(t)}
                disabled={busy}
                className="shrink-0 text-[10px] text-red-400 hover:underline"
              >
                删除
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
          placeholder="Token 名称，如 PicGo"
          className="flex-1 rounded-lg bg-neutral-900 border border-neutral-800 h-8 px-2.5 text-xs outline-none focus:border-brand-500 placeholder-faint"
        />
        <select
          value={days}
          onChange={(e) => setDays(Number(e.target.value))}
          className="h-8 rounded-lg bg-neutral-900 border border-neutral-800 px-2 text-xs outline-none focus:border-brand-500"
        >
          <option value={0}>永不过期</option>
          <option value={30}>30 天</option>
          <option value={90}>90 天</option>
          <option value={365}>1 年</option>
        </select>
        <button
          onClick={create}
          disabled={busy || !name.trim()}
          className="inline-flex h-8 items-center justify-center rounded-lg bg-brand-600 px-3 text-xs font-medium text-brand-ink hover:bg-brand-500 disabled:bg-neutral-800 disabled:text-neutral-600 transition whitespace-nowrap"
        >
          生成
        </button>
      </div>

      <p className="mt-2 text-[10px] text-neutral-600">
        Token 只能用于上传和管理图片，不能修改账号设置、存储凭据或 Passkey。
      </p>
    </div>
  );
}
