import { useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../AuthContext";
import { Field, inputCls } from "./Login";

export default function EmailOtpForm() {
  const { requestOtp, verifyOtp } = useAuth();
  const nav = useNavigate();
  const loc = useLocation();
  const next = (loc.state as { from?: string } | null)?.from || "/";

  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [stage, setStage] = useState<"email" | "code">("email");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);
  const [resendIn, setResendIn] = useState(0);

  useEffect(() => {
    if (resendIn <= 0) return;
    const t = setInterval(() => setResendIn((s) => Math.max(0, s - 1)), 1000);
    return () => clearInterval(t);
  }, [resendIn]);

  async function onRequest(e: React.FormEvent) {
    e.preventDefault();
    if (busy || !email) return;
    setBusy(true); setErr(null); setInfo(null);
    try {
      await requestOtp(email);
      setStage("code");
      setInfo(`已发送 6 位验证码到 ${email}`);
      setResendIn(60);
    } catch (e) {
      setErr(String(e instanceof Error ? e.message : e));
    } finally {
      setBusy(false);
    }
  }

  async function onVerify(e: React.FormEvent) {
    e.preventDefault();
    if (busy || !code) return;
    setBusy(true); setErr(null);
    try {
      await verifyOtp(email, code);
      nav(next, { replace: true });
    } catch (e) {
      setErr(String(e instanceof Error ? e.message : e));
    } finally {
      setBusy(false);
    }
  }

  if (stage === "email") {
    return (
      <form onSubmit={onRequest} className="space-y-4">
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
        {err && <div className="text-sm text-red-400">{err}</div>}
        <button
          type="submit"
          disabled={busy || !email}
          className="w-full rounded-lg bg-brand-600 px-4 py-3 text-sm font-medium text-brand-ink hover:bg-brand-500 disabled:bg-neutral-700 disabled:text-neutral-500"
        >
          {busy ? "发送中…" : "发送验证码"}
        </button>
        <p className="text-xs text-neutral-500 text-center">
          没账号也行，验证后自动创建
        </p>
      </form>
    );
  }

  return (
    <form onSubmit={onVerify} className="space-y-4">
      {info && <div className="text-sm text-emerald-400">{info}</div>}
      <Field label="验证码">
        <input
          inputMode="numeric"
          pattern="[0-9]{6}"
          maxLength={6}
          required
          autoFocus
          value={code}
          onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
          className={`${inputCls} text-center text-2xl tracking-[0.5em] font-mono`}
        />
      </Field>
      {err && <div className="text-sm text-red-400">{err}</div>}
      <button
        type="submit"
        disabled={busy || code.length !== 6}
        className="w-full rounded-lg bg-brand-600 px-4 py-3 text-sm font-medium text-brand-ink hover:bg-brand-500 disabled:bg-neutral-700 disabled:text-neutral-500"
      >
        {busy ? "验证中…" : "登录"}
      </button>
      <div className="flex items-center justify-between text-xs text-neutral-500">
        <button type="button" onClick={() => { setStage("email"); setCode(""); setErr(null); }} className="hover:text-neutral-300">
          ← 改邮箱
        </button>
        {resendIn > 0 ? (
          <span>{resendIn}s 后可重发</span>
        ) : (
          <button
            type="button"
            disabled={busy}
            onClick={async () => {
              setBusy(true); setErr(null);
              try {
                await requestOtp(email);
                setInfo(`重新发送到 ${email}`);
                setResendIn(60);
              } catch (e) {
                setErr(String(e instanceof Error ? e.message : e));
              } finally {
                setBusy(false);
              }
            }}
            className="text-brand-400 hover:underline"
          >
            重新发送
          </button>
        )}
      </div>
    </form>
  );
}
