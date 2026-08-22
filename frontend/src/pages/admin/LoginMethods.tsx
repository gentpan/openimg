import Icon from "../../Icon";
import { useEffect, useState } from "react";
import { adminApi } from "../../api";
import type { OAuthStatus } from "../../types";
import { RingSpinner } from "../../components/Spinner";

/**
 * OAuth setup lives here rather than only in the env file because of a
 * chicken-and-egg problem: Google won't give you a client ID until you've
 * registered a redirect URI, and you can't know the redirect URI until the
 * site is deployed. So we show the exact URI to paste, then take the
 * credentials back without a redeploy.
 */
export default function LoginMethods() {
  const [status, setStatus] = useState<OAuthStatus | null>(null);
  const [msg, setMsg] = useState<{ kind: "ok" | "err"; text: string } | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [form, setForm] = useState<Record<string, { id: string; secret: string }>>({
    google: { id: "", secret: "" },
    github: { id: "", secret: "" },
  });

  async function load() {
    const s = await adminApi.oauthStatus();
    setStatus(s);
    setForm({
      google: { id: s.google.client_id, secret: "" },
      github: { id: s.github.client_id, secret: "" },
    });
  }
  useEffect(() => {
    load().catch((e) => setMsg({ kind: "err", text: String(e.message || e) }));
  }, []);

  async function save(provider: "google" | "github") {
    setBusy(provider);
    setMsg(null);
    try {
      await adminApi.oauthSave(provider, form[provider].id, form[provider].secret);
      await load();
      setMsg({ kind: "ok", text: `${provider} 配置已保存，登录页会立即出现按钮` });
    } catch (e) {
      setMsg({ kind: "err", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(null);
    }
  }

  if (!status) return <div className="text-xs text-neutral-600">加载中…</div>;

  return (
    <div className="space-y-3">
      {msg && (
        <div
          className={`rounded-xl px-4 py-2.5 text-xs ${
            msg.kind === "ok"
              ? "border border-teal-500/30 bg-teal-950/20 text-teal-200"
              : "border border-red-500/30 bg-red-950/20 text-red-200"
          }`}
        >
          {msg.text}
        </div>
      )}

      {!status.can_store && (
        <div className="rounded-xl border border-amber-500/30 bg-amber-950/20 px-4 py-2.5 text-xs text-amber-200">
          <Icon name="triangle-exclamation" className="mr-1.5"  />
          服务端未配置 STORAGE_MASTER_KEY，无法在后台加密保存密钥。请改用环境变量，或先生成主密钥：
          <code className="ml-1 text-amber-100">openssl rand -base64 32</code>
        </div>
      )}

      <div className="grid lg:grid-cols-2 gap-3">
        {(["google", "github"] as const).map((p) => {
          const info = status[p];
          const label = p === "google" ? "Google" : "GitHub";
          return (
            <div key={p} className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-4">
              <div className="flex items-center gap-2 mb-3">
                <span className="text-sm text-neutral-100">{label} 登录</span>
                {info.enabled ? (
                  <>
                    <span className="rounded-full bg-teal-900/50 px-1.5 py-0.5 text-[9px] text-teal-300">
                      已启用
                    </span>
                    <span className="rounded-full bg-neutral-800 px-1.5 py-0.5 text-[9px] text-neutral-400">
                      {info.source === "env" ? "环境变量" : "后台配置"}
                    </span>
                  </>
                ) : (
                  <span className="rounded-full bg-neutral-800 px-1.5 py-0.5 text-[9px] text-neutral-500">
                    未配置
                  </span>
                )}
                <div className="flex-1" />
                <a
                  href={info.console_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-[10px] text-brand-400 hover:underline"
                >
                  去申请 <Icon name="arrow-up-right-from-square" className="text-[8px]"  />
                </a>
              </div>

              <div className="mb-3">
                <div className="text-[10px] text-neutral-600 mb-1">
                  回调地址（复制到 {label} 控制台的 Redirect URI）
                </div>
                <CopyRow value={info.redirect_uri} />
              </div>

              <div className="space-y-2">
                <Field
                  label="Client ID"
                  value={form[p].id}
                  onChange={(v) => setForm({ ...form, [p]: { ...form[p], id: v } })}
                  placeholder="留空表示停用"
                />
                <Field
                  label="Client Secret"
                  type="password"
                  value={form[p].secret}
                  onChange={(v) => setForm({ ...form, [p]: { ...form[p], secret: v } })}
                  placeholder={info.secret_state || "必填"}
                />
                {info.secret_state && (
                  <div className="text-[10px] text-neutral-600">
                    当前：{info.secret_state}（留空则保持不变，填 <code>-</code> 清除）
                  </div>
                )}
              </div>

              {info.source === "env" && (
                <div className="mt-2 rounded-lg border border-neutral-800 bg-neutral-950/40 px-2.5 py-2 text-[10px] leading-relaxed text-neutral-500">
                  当前凭据来自 <code className="text-neutral-300">.env</code>。在这里保存会写入数据库并
                  <span className="text-amber-300">覆盖环境变量</span>——两处都配时以数据库为准。
                  想继续用 .env 管理就别在这里保存。
                </div>
              )}

              <button
                onClick={() => save(p)}
                disabled={busy === p || !status.can_store}
                className="mt-3 rounded-lg bg-brand-600 px-3 py-1.5 text-xs font-medium text-brand-ink hover:bg-brand-500 disabled:bg-neutral-800 disabled:text-neutral-600 transition"
              >
                {busy === p ? <RingSpinner className="h-3.5 w-3.5 inline-block align-[-2px]" /> : "保存"}
              </button>
            </div>
          );
        })}
      </div>

      <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-4">
        <div className="text-sm text-neutral-100 mb-3">其他登录方式</div>
        <div className="space-y-2">
          <MethodRow
            label="Passkey / 生物识别"
            enabled={status.passkey.enabled}
            note={`RP ID 绑定在 ${new URL(status.base_url).hostname}，更换域名后已注册的 Passkey 会失效`}
          />
          <MethodRow
            label="邮箱验证码"
            enabled={status.email_otp.enabled}
            note="需要配置 SENDFLARE_API_KEY 环境变量"
          />
          <MethodRow label="邮箱 + 密码" enabled note="始终可用" />
        </div>
      </div>
    </div>
  );
}

function MethodRow({ label, enabled, note }: { label: string; enabled: boolean; note: string }) {
  return (
    <div className="flex items-center gap-3">
      <Icon
        name={enabled ? "circle-check" : "circle-xmark"}
        className={`${enabled ? "text-teal-400" : "text-faint"} text-xs`}
      />
      <div className="flex-1 min-w-0">
        <div className="text-xs text-neutral-300">{label}</div>
        <div className="text-[10px] text-neutral-600">{note}</div>
      </div>
      <span className={`text-[10px] ${enabled ? "text-teal-400" : "text-neutral-600"}`}>
        {enabled ? "已启用" : "未启用"}
      </span>
    </div>
  );
}

function CopyRow({ value }: { value: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="flex items-center gap-1.5">
      <input
        readOnly
        value={value}
        onFocus={(e) => e.currentTarget.select()}
        className="flex-1 min-w-0 rounded-md bg-neutral-950 border border-neutral-800 px-2 py-1.5 text-[11px] font-mono text-neutral-300 outline-none"
      />
      <button
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(value);
            setCopied(true);
            setTimeout(() => setCopied(false), 1500);
          } catch {}
        }}
        className={`shrink-0 w-7 h-7 rounded-md text-[10px] transition ${
          copied ? "bg-brand-600 text-brand-ink" : "bg-neutral-800 text-neutral-300 hover:bg-neutral-700"
        }`}
      >
        <Icon name={copied ? "check" : "copy"}  />
      </button>
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  placeholder,
  type = "text",
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  type?: string;
}) {
  return (
    <div>
      <label className="block text-[10px] text-neutral-600 mb-1">{label}</label>
      <input
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg bg-neutral-900 border border-neutral-800 px-2.5 py-1.5 text-xs outline-none focus:border-brand-500 placeholder-faint"
      />
    </div>
  );
}
