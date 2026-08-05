import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../AuthContext";
import { authApi } from "../api";
import PasswordField from "../components/PasswordField";
import { AuthShell, Field, inputCls } from "./Login";
import OAuthButtons from "./OAuthButtons";

export default function RegisterPage() {
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
      setErr("请先填写邮箱");
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
    <AuthShell title="注册">
      <form onSubmit={onSubmit} className="space-y-4">
        <Field label="邮箱">
          <input type="email" required value={email} onChange={(e) => setEmail(e.target.value)} className={inputCls} autoComplete="email" />
        </Field>
        <Field label="昵称">
          <input
            value={name}
            required
            maxLength={32}
            placeholder="别人看到的名字，可重复"
            onChange={(e) => setName(e.target.value)}
            className={inputCls}
            autoComplete="nickname"
          />
        </Field>
        <Field label="密码（至少 8 位）">
          <PasswordField value={password} onChange={setPassword} />
        </Field>
        <Field label="确认密码">
          <PasswordField
            value={password2}
            onChange={setPassword2}
            placeholder="再输入一次"
            showStrength={false}
            showGenerate={false}
          />
          {mismatch && <div className="mt-1.5 text-[11px] text-red-400">两次输入不一致</div>}
        </Field>
        <Field label="邮箱验证码">
          <div className="flex gap-1.5">
            <input
              value={code}
              inputMode="numeric"
              maxLength={6}
              required
              autoComplete="one-time-code"
              placeholder="6 位数字"
              onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
              className={`${inputCls} flex-1 min-w-0 tabular-nums`}
            />
            <button
              type="button"
              onClick={sendCode}
              disabled={sending || cooldown > 0}
              className="shrink-0 rounded-lg bg-neutral-800 px-3 text-xs text-neutral-300 transition hover:bg-neutral-700 disabled:text-neutral-500 disabled:hover:bg-neutral-800"
            >
              {sending ? "发送中…" : cooldown > 0 ? `${cooldown}s` : sentTo ? "重新发送" : "发送验证码"}
            </button>
          </div>
          {sentTo && (
            <div className="mt-1.5 text-[11px] text-neutral-500">
              验证码已发送到 <span className="text-neutral-300">{sentTo}</span>，5 分钟内有效
            </div>
          )}
        </Field>
        {err && <div className="text-sm text-red-400">{err}</div>}
        <button
          type="submit"
          disabled={busy || !ready}
          className="w-full rounded-lg bg-brand-600 px-4 py-3 text-sm font-medium text-brand-ink transition hover:bg-brand-500 disabled:bg-neutral-800 disabled:text-neutral-500"
        >
          {busy ? "注册中…" : "创建账号"}
        </button>
      </form>
      <div className="mt-6">
        <OAuthButtons />
      </div>
      <div className="mt-5 text-sm text-neutral-500 text-center">
        已有账号？{" "}
        <Link to="/login" className="text-brand-400 hover:underline">
          登录
        </Link>
      </div>
    </AuthShell>
  );
}
