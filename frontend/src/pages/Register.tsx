import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../AuthContext";
import { authApi } from "../api";
import PasswordField from "../components/PasswordField";
import { AuthShell, Field, inputCls } from "./Login";
import OAuthButtons from "./OAuthButtons";
import { useLang } from "../LangContext";

export default function RegisterPage() {
  const { t } = useLang();
  const { register } = useAuth();
  const nav = useNavigate();
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [password2, setPassword2] = useState("");
  const [code, setCode] = useState("");
  const [sending, setSending] = useState(false);
  const [sentTo, setSentTo] = useState("");
  const [cooldown, setCooldown] = useState(0);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (cooldown <= 0) return;
    const t = setTimeout(() => setCooldown((n) => n - 1), 1000);
    return () => clearTimeout(t);
  }, [cooldown]);

  async function sendCode() {
    if (!email.includes("@")) {
      setErr(t.auth.register.emailRequiredFirst);
      return;
    }
    setSending(true);
    setErr(null);
    try {
      const r = await authApi.registerCode(email);
      setSentTo(email);
      setCooldown(r.resend_in);
    } catch (e) {
      setErr(String(e instanceof Error ? e.message : e));
    } finally {
      setSending(false);
    }
  }

  const mismatch = password2.length > 0 && password !== password2;
  const ready =
    name.trim().length > 0 &&
    password.length >= 8 &&
    password === password2 &&
    code.length === 6;

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!ready) return;
    setBusy(true);
    setErr(null);
    try {
      await register(email, password, code, name || undefined);
      nav("/", { replace: true });
    } catch (e) {
      setErr(String(e instanceof Error ? e.message : e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthShell title={t.auth.shell.signUp}>
      <form onSubmit={onSubmit} className="space-y-4">
        <Field label={t.common.email}>
          <input type="email" required value={email} onChange={(e) => setEmail(e.target.value)} className={inputCls} autoComplete="email" />
        </Field>
        <Field label={t.common.displayName}>
          <input
            value={name}
            required
            maxLength={32}
            placeholder={t.auth.register.displayNamePlaceholder}
            onChange={(e) => setName(e.target.value)}
            className={inputCls}
            autoComplete="nickname"
          />
        </Field>
        <Field label={t.auth.register.password}>
          <PasswordField value={password} onChange={setPassword} />
        </Field>
        <Field label={t.auth.register.confirmPassword}>
          <PasswordField
            value={password2}
            onChange={setPassword2}
            placeholder={t.common.enterAgain}
            showStrength={false}
            showGenerate={false}
          />
          {mismatch && <div className="mt-1.5 text-[11px] text-red-400">{t.common.passwordMismatch}</div>}
        </Field>
        <Field label={t.common.emailCode}>
          <div className="flex gap-1.5">
            <input
              value={code}
              inputMode="numeric"
              maxLength={6}
              required
              autoComplete="one-time-code"
              placeholder={t.common.otpPlaceholder}
              onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
              className={`${inputCls} flex-1 min-w-0 tabular-nums`}
            />
            <button
              type="button"
              onClick={sendCode}
              disabled={sending || cooldown > 0}
              className="shrink-0 rounded-lg bg-neutral-800 px-3 text-xs text-neutral-300 transition hover:bg-neutral-700 disabled:text-neutral-500 disabled:hover:bg-neutral-800"
            >
              {sending ? t.common.sendingCode : cooldown > 0 ? `${cooldown}s` : sentTo ? t.common.resend : t.common.sendCode}
            </button>
          </div>
          {sentTo && (
            <div className="mt-1.5 text-[11px] text-neutral-500">
              {t.common
                .otpSentTo(sentTo)
                .split(sentTo)
                .flatMap((part, i) =>
                  i === 0
                    ? [part]
                    : [
                        <span key={i} className="text-neutral-300">
                          {sentTo}
                        </span>,
                        part,
                      ],
                )}
            </div>
          )}
        </Field>
        {err && <div className="text-sm text-red-400">{err}</div>}
        <button
          type="submit"
          disabled={busy || !ready}
          className="w-full rounded-lg bg-brand-600 px-4 py-3 text-sm font-medium text-brand-ink transition hover:bg-brand-500 disabled:bg-neutral-800 disabled:text-neutral-500"
        >
          {busy ? t.auth.register.submitBusy : t.auth.register.submit}
        </button>
      </form>
      <div className="mt-6">
        <OAuthButtons />
      </div>
      <div className="mt-5 text-sm text-neutral-500 text-center">
        {t.auth.haveAccountPrompt}{" "}
        <Link to="/login" className="text-brand-400 hover:underline">
          {t.common.signIn}
        </Link>
      </div>
    </AuthShell>
  );
}
