import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../AuthContext";
import Logo from "../Logo";
import PasswordField from "../components/PasswordField";
import { useTheme } from "../ThemeContext";
import { authApi, resetApi } from "../api";
import PasskeyLoginForm from "./PasskeyLoginForm";
import OAuthButtons from "./OAuthButtons";

type Mode = "password" | "otp" | "passkey";

export default function LoginPage() {
  const { login } = useAuth();
  const nav = useNavigate();
  const loc = useLocation();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [mode, setMode] = useState<Mode>("password");
  const [reset, setReset] = useState(false);
  const [providers, setProviders] = useState<string[]>([]);
  const next = (loc.state as { from?: string } | null)?.from || "/";

  useEffect(() => {
    authApi.providers().then(setProviders).catch(() => {});
  }, []);
  const passkeyAvailable = providers.includes("passkey");

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      await login(email, password);
      nav(next, { replace: true });
    } catch (e) {
      setErr(String(e instanceof Error ? e.message : e));
    } finally {
      setBusy(false);
    }
  }

  if (reset) return <ResetPassword onDone={() => setReset(false)} />;

  return (
    <AuthShell title="登录">
      {passkeyAvailable && (
        <div className="mb-5 flex gap-1 rounded-lg bg-neutral-950 p-1 text-xs">
          <ModeBtn active={mode === "password"} onClick={() => setMode("password")}>
            邮箱 + 密码
          </ModeBtn>
          <ModeBtn active={mode === "passkey"} onClick={() => setMode("passkey")}>
            Passkey
          </ModeBtn>
        </div>
      )}

      {mode === "passkey" ? (
        <PasskeyLoginForm />
      ) : (
        <form onSubmit={onSubmit} className="space-y-4">
          <Field label="邮箱">
            <input
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoComplete="email"
              className={inputCls}
            />
          </Field>
          <Field label="密码">
            <input
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              className={inputCls}
            />
          </Field>
          {err && <div className="text-sm text-red-400">{err}</div>}
          <div className="text-right">
            <button
              type="button"
              onClick={() => setReset(true)}
              className="text-xs text-neutral-500 hover:text-violet-300 transition"
            >
              忘记密码？
            </button>
          </div>
          <button
            type="submit"
            disabled={busy}
            className="w-full rounded-lg bg-violet-600 px-4 py-3 text-sm font-medium text-white hover:bg-violet-500 disabled:bg-neutral-700"
          >
            {busy ? "登录中…" : "登录"}
          </button>
        </form>
      )}

      <div className="mt-6">
        <OAuthButtons onPasskey={() => setMode("passkey")} />
      </div>

      <div className="mt-5 text-sm text-neutral-500 text-center">
        没账号？{" "}
        <Link to="/register" className="text-violet-400 hover:underline">
          注册
        </Link>
      </div>
    </AuthShell>
  );
}

function ModeBtn({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex-1 rounded-md px-3 py-1.5 transition ${
        active ? "bg-violet-600/20 text-violet-300" : "text-neutral-500 hover:text-neutral-300"
      }`}
    >
      {children}
    </button>
  );
}

export function AuthShell({ title, children }: { title: string; children: React.ReactNode }) {
  // These pages don't render <Nav>, so without this the theme is unreachable
  // for anyone who hasn't signed in yet.
  const { theme, toggle } = useTheme();
  return (
    <div className="relative min-h-screen flex flex-col items-center justify-center px-4">
      <button
        onClick={toggle}
        title={theme === "dark" ? "切换到浅色主题" : "切换到深色主题"}
        aria-label={theme === "dark" ? "切换到浅色主题" : "切换到深色主题"}
        className="absolute top-4 right-4 w-8 h-8 rounded-full text-neutral-500 hover:text-violet-300 hover:bg-neutral-900 transition"
      >
        <i className={`fa-solid ${theme === "dark" ? "fa-sun" : "fa-moon"} text-xs`} />
      </button>
      <div className="w-full max-w-sm rounded-2xl border border-neutral-800 bg-neutral-900/60 p-8">
        <div className="mb-6 flex items-center gap-3">
          <Logo size={36} />
          <div>
            <h1 className="text-xl font-brand leading-tight">
              {title} Open<span className="text-violet-400">img</span>
            </h1>
          </div>
        </div>
        {children}
      </div>
      <Link
        to="/"
        className="mt-4 inline-flex items-center gap-1 text-xs text-neutral-500 hover:text-neutral-300 transition"
      >
        <span aria-hidden>←</span> 返回首页
      </Link>
    </div>
  );
}

export function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <div className="mb-1 text-xs text-neutral-400">{label}</div>
      {children}
    </label>
  );
}

export const inputCls =
  "w-full rounded-lg border border-neutral-800 bg-neutral-950 px-3 py-2 text-sm placeholder-neutral-600 outline-none focus:border-violet-500";

/**
 * Password recovery.
 *
 * Not a third login mode — it mints a session only after a new password has
 * been set. It exists because email+password is now one of two ways in, and
 * accounts created by the old magic-link flow have no password at all; without
 * this they would be locked out for good.
 */
function ResetPassword({ onDone }: { onDone: () => void }) {
  const nav = useNavigate();
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [pw, setPw] = useState("");
  const [sending, setSending] = useState(false);
  const [cooldown, setCooldown] = useState(0);
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (cooldown <= 0) return;
    const t = setTimeout(() => setCooldown((n) => n - 1), 1000);
    return () => clearTimeout(t);
  }, [cooldown]);

  async function send() {
    if (!email.includes("@")) {
      setErr("请输入邮箱");
      return;
    }
    setSending(true);
    setErr(null);
    try {
      const r = await resetApi.request(email);
      setSent(true);
      setCooldown(r.resend_in);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setSending(false);
    }
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      await resetApi.confirm(email, code, pw);
      nav("/dashboard", { replace: true });
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthShell title="重置">
      <form onSubmit={submit} className="space-y-4">
        <Field label="邮箱">
          <input
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
            className={inputCls}
          />
        </Field>

        <Field label="邮箱验证码">
          <div className="flex gap-1.5">
            <input
              value={code}
              inputMode="numeric"
              maxLength={6}
              autoComplete="one-time-code"
              placeholder="6 位数字"
              onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
              className={`${inputCls} flex-1 min-w-0 tabular-nums`}
            />
            <button
              type="button"
              onClick={send}
              disabled={sending || cooldown > 0}
              className="shrink-0 rounded-lg bg-neutral-800 px-3 text-xs text-neutral-300 hover:bg-neutral-700 disabled:text-neutral-500 disabled:hover:bg-neutral-800 transition"
            >
              {sending ? "发送中…" : cooldown > 0 ? `${cooldown}s` : sent ? "重新发送" : "发送验证码"}
            </button>
          </div>
        </Field>

        <Field label="新密码">
          <PasswordField value={pw} onChange={setPw} />
        </Field>

        {sent && (
          <div className="text-xs text-neutral-500">
            如果这个邮箱已注册，验证码已经发出。为避免泄露账号是否存在，我们不区分两种情况。
          </div>
        )}
        {err && <div className="text-sm text-red-400">{err}</div>}

        <button
          type="submit"
          disabled={busy || code.length !== 6 || pw.length < 8}
          className="w-full rounded-lg bg-violet-600 px-4 py-3 text-sm font-medium text-white hover:bg-violet-500 disabled:bg-neutral-800 disabled:text-neutral-500"
        >
          {busy ? "设置中…" : "设置新密码并登录"}
        </button>
      </form>

      <div className="mt-5 text-center">
        <button onClick={onDone} className="text-sm text-neutral-500 hover:text-neutral-300 transition">
          ← 返回登录
        </button>
      </div>
    </AuthShell>
  );
}
