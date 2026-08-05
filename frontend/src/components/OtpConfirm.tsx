import { useEffect, useRef, useState } from "react";
import { accountApi, type OtpPurpose } from "../api";
import { RingSpinner } from "./Spinner";

const LABELS: Record<OtpPurpose, { title: string; action: string }> = {
  password: { title: "修改密码", action: "确认修改" },
  passkey: { title: "添加 Passkey", action: "继续添加" },
  purge: { title: "清空图库", action: "确认清空" },
};

/**
 * Confirmation dialog for actions gated on an emailed code.
 *
 * It requests the code itself on open rather than making the caller do it, so
 * every gated action gets the same flow: open, read the mail, type six digits.
 * The address is never passed in — the server mails whatever is on the session,
 * which is what makes this a check rather than a formality.
 */
export default function OtpConfirm({
  purpose,
  detail,
  danger,
  onCancel,
  onVerified,
}: {
  purpose: OtpPurpose;
  /** Extra line describing exactly what is about to happen. */
  detail?: React.ReactNode;
  danger?: boolean;
  onCancel: () => void;
  /** Receives the code; throw to keep the dialog open and show the message. */
  onVerified: (code: string) => Promise<void>;
}) {
  const [code, setCode] = useState("");
  const [sending, setSending] = useState(true);
  const [busy, setBusy] = useState(false);
  const [sentTo, setSentTo] = useState("");
  const [cooldown, setCooldown] = useState(0);
  const [err, setErr] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  // React 19 StrictMode double-invokes effects in development; without this
  // the dialog would mail two codes and the first would be the stale one.
  const requested = useRef(false);

  async function send() {
    setSending(true);
    setErr(null);
    try {
      const r = await accountApi.requestOtp(purpose);
      setSentTo(r.email);
      setCooldown(r.resend_in);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setSending(false);
      inputRef.current?.focus();
    }
  }

  useEffect(() => {
    if (requested.current) return;
    requested.current = true;
    send();
  }, []);

  useEffect(() => {
    if (cooldown <= 0) return;
    const t = setTimeout(() => setCooldown((n) => n - 1), 1000);
    return () => clearTimeout(t);
  }, [cooldown]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && !busy && onCancel();
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [busy, onCancel]);

  async function submit() {
    if (code.length !== 6 || busy) return;
    setBusy(true);
    setErr(null);
    try {
      await onVerified(code);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
      setCode("");
      inputRef.current?.focus();
    } finally {
      setBusy(false);
    }
  }

  const label = LABELS[purpose];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center px-4">
      <div className="absolute inset-0 bg-black/50" onClick={() => !busy && onCancel()} />
      <div className="relative w-full max-w-sm rounded-2xl border border-neutral-800 bg-neutral-900 p-5 shadow-panel">
        <div className="flex items-center gap-2 mb-1">
          <span
            className={`w-7 h-7 rounded-lg flex items-center justify-center text-xs ${
              danger ? "bg-red-900/40 text-red-300" : "bg-brand-900/40 text-brand-300"
            }`}
          >
            <i className="fa-solid fa-envelope-circle-check" />
          </span>
          <span className="text-sm text-neutral-100">{label.title}</span>
        </div>

        <p className="text-[11px] leading-relaxed text-neutral-500 mb-3">
          {sending ? (
            "正在发送验证码…"
          ) : sentTo ? (
            <>
              验证码已发送到 <span className="text-neutral-300">{sentTo}</span>，5 分钟内有效。
            </>
          ) : (
            "无法发送验证码。"
          )}
          {detail && <span className="block mt-1.5">{detail}</span>}
        </p>

        <input
          ref={inputRef}
          value={code}
          inputMode="numeric"
          autoComplete="one-time-code"
          maxLength={6}
          placeholder="6 位验证码"
          onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
          onKeyDown={(e) => e.key === "Enter" && submit()}
          className="w-full rounded-lg bg-neutral-950 border border-neutral-800 px-3 py-2 text-center text-lg tracking-[0.4em] tabular-nums outline-none focus:border-brand-500 placeholder-faint placeholder:text-sm placeholder:tracking-normal"
        />

        {err && <div className="mt-2 text-[11px] text-red-400">{err}</div>}

        <div className="mt-3 flex items-center gap-2">
          <button
            onClick={send}
            disabled={sending || cooldown > 0 || busy}
            className="text-[11px] text-neutral-500 hover:text-brand-300 disabled:hover:text-neutral-500 disabled:opacity-60 transition"
          >
            {cooldown > 0 ? `重新发送 (${cooldown}s)` : "重新发送"}
          </button>
          <div className="flex-1" />
          <button
            onClick={onCancel}
            disabled={busy}
            className="rounded-lg px-3 py-1.5 text-xs text-neutral-400 hover:text-neutral-100 transition"
          >
            取消
          </button>
          <button
            onClick={submit}
            disabled={code.length !== 6 || busy}
            className={`rounded-lg px-3 py-1.5 text-xs font-medium text-white transition disabled:bg-neutral-800 disabled:text-neutral-500 ${
              danger ? "bg-red-600 hover:bg-red-700" : "bg-brand-600 hover:bg-brand-500"
            }`}
          >
            {busy ? <RingSpinner className="h-3.5 w-3.5 inline-block align-[-2px]" /> : label.action}
          </button>
        </div>
      </div>
    </div>
  );
}
