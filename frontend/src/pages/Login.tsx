import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../AuthContext";
import Logo from "../Logo";
import PasswordField from "../components/PasswordField";
import { authApi, resetApi, nativeAuth } from "../api";
import PasskeyLoginForm from "./PasskeyLoginForm";
import OAuthButtons from "./OAuthButtons";
import { useLang } from "../LangContext";



export default function LoginPage() {
  const { t } = useLang();
  const { login, user } = useAuth();
  const nav = useNavigate();
  const loc = useLocation();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  type Mode = "password" | "passkey";

const [mode, setMode] = useState<Mode>("password");
  const [reset, setReset] = useState(false);
  const [providers, setProviders] = useState<string[]>([]);
  const next = (loc.state as { from?: string } | null)?.from || "/";
  // The Mac app opens this page inside a web sheet. Set on the URL rather than
  // handled per login method, because passkey, OTP and OAuth each finish on
  // their own path — hooking every one of them is how a method gets forgotten.
  const native = new URLSearchParams(loc.search).get("native") === "1";

  useEffect(() => {
    authApi.providers().then(setProviders).catch(() => {});
  }, []);

  // One exit for every method: as soon as a session exists, trade it for a
  // one-time code and hand that to the app. The session itself stays in the
  // sheet and dies with it.
  useEffect(() => {
    if (!native || !user) return;
    nativeAuth
      .code()
      .then(({ code }) => {
        window.location.href = `openimg://auth?code=${encodeURIComponent(code)}`;
      })
      .catch(() => setErr(t.auth.nativeHandoffFailed));
  }, [native, user]);
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
    <AuthShell title={t.auth.shell.signIn}>
      {passkeyAvailable && (
        <div className="mb-5 flex gap-1 rounded-lg bg-neutral-950 p-1 text-xs">
          <ModeBtn active={mode === "password"} onClick={() => setMode("password")}>
            {t.common.emailPassword}
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
          <Field label={t.common.email}>
            <input
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoComplete="email"
              className={inputCls}
            />
          </Field>
          <Field label={t.auth.password}>
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
              className="text-xs text-neutral-500 hover:text-brand-300 transition"
            >
              {t.auth.forgotPassword}
            </button>
          </div>
          <button
            type="submit"
            disabled={busy}
            className="w-full rounded-lg bg-brand-600 px-4 py-3 text-sm font-medium text-brand-ink hover:bg-brand-500 disabled:bg-neutral-700 disabled:text-neutral-500"
          >
            {busy ? t.auth.signingIn : t.common.signIn}
          </button>
        </form>
      )}

      <div className="mt-6">
        <OAuthButtons onPasskey={() => setMode("passkey")} />
      </div>

      <div className="mt-5 text-sm text-neutral-500 text-center">
        {t.auth.noAccountPrompt}{" "}
        <Link to="/register" className="text-brand-400 hover:underline">
          {t.common.signUp}
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
        active ? "bg-brand-600/20 text-brand-300" : "text-neutral-500 hover:text-neutral-300"
      }`}
    >
      {children}
    </button>
  );
}

export function AuthShell({ title, children }: { title: string; children: React.ReactNode }) {
  const { t } = useLang();
  // These pages don't render <Nav>, so without this the theme is unreachable
  // for anyone who hasn't signed in yet.
  
  return (
    <div className="relative min-h-screen flex flex-col items-center justify-center px-4">
      <div className="w-full max-w-sm rounded-2xl border border-neutral-800 bg-neutral-900/60 p-8">
        <div className="mb-6 flex items-center gap-3">
          <Logo size={36} />
          <div>
            <h1 className="text-xl font-brand leading-tight">
              {title} Open<span className="text-brand-400">img</span>
            </h1>
          </div>
        </div>
        {children}
      </div>
      <Link
        to="/"
        className="mt-4 inline-flex items-center gap-1 text-xs text-neutral-500 hover:text-neutral-300 transition"
      >
        <span aria-hidden>←</span> {t.common.backToHome}
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
  "w-full rounded-lg border border-neutral-800 bg-neutral-950 px-3 py-2 text-sm placeholder-neutral-600 outline-none focus:border-brand-500";

/**
 * Password recovery.
 *
 * Not a third login mode — it mints a session only after a new password has
 * been set. It exists because email+password is now one of two ways in, and
 * accounts created by the old magic-link flow have no password at all; without
 * this they would be locked out for good.
 */
function ResetPassword({ onDone }: { onDone: () => void }) {
  const { t } = useLang();
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
      setErr(t.auth.reset.emailRequired);
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
    <AuthShell title={t.auth.shell.reset}>
      <form onSubmit={submit} className="space-y-4">
        <Field label={t.common.email}>
          <input
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
            className={inputCls}
          />
        </Field>

        <Field label={t.common.emailCode}>
          <div className="flex gap-1.5">
            <input
              value={code}
              inputMode="numeric"
              maxLength={6}
              autoComplete="one-time-code"
              placeholder={t.common.otpPlaceholder}
              onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
              className={`${inputCls} flex-1 min-w-0 tabular-nums`}
            />
            <button
              type="button"
              onClick={send}
              disabled={sending || cooldown > 0}
              className="shrink-0 rounded-lg bg-neutral-800 px-3 text-xs text-neutral-300 hover:bg-neutral-700 disabled:text-neutral-500 disabled:hover:bg-neutral-800 transition"
            >
              {sending ? t.common.sendingCode : cooldown > 0 ? `${cooldown}s` : sent ? t.common.resend : t.common.sendCode}
            </button>
          </div>
        </Field>

        <Field label={t.common.newPassword}>
          <PasswordField value={pw} onChange={setPw} />
        </Field>

        {sent && (
          <div className="text-xs text-neutral-500">
            {t.auth.resetCodeSentNeutral}
          </div>
        )}
        {err && <div className="text-sm text-red-400">{err}</div>}

        <button
          type="submit"
          disabled={busy || code.length !== 6 || pw.length < 8}
          className="w-full rounded-lg bg-brand-600 px-4 py-3 text-sm font-medium text-brand-ink hover:bg-brand-500 disabled:bg-neutral-800 disabled:text-neutral-500"
        >
          {busy ? t.auth.reset.submitBusy : t.auth.reset.submit}
        </button>
      </form>

      <div className="mt-5 text-center">
        <button onClick={onDone} className="text-sm text-neutral-500 hover:text-neutral-300 transition">
          {t.auth.reset.backToSignIn}
        </button>
      </div>
    </AuthShell>
  );
}
