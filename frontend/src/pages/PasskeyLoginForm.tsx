import Icon from "../Icon";
import { startAuthentication } from "@simplewebauthn/browser";
import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { authApi } from "../api";
import { useAuth } from "../AuthContext";
import { Field, inputCls } from "./Login";
import { useLang } from "../LangContext";

export default function PasskeyLoginForm() {
  const { t } = useLang();
  const { refresh } = useAuth();
  const nav = useNavigate();
  const loc = useLocation();
  const next = (loc.state as { from?: string } | null)?.from || "/";

  const [email, setEmail] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setErr(null);
    try {
      // Empty email triggers discoverable login: browser opens the platform
      // passkey picker, user picks their passkey, the credential reveals which
      // account to log in. No email guessing required.
      const begin = await authApi.passkeyLoginBegin(email || undefined);
      const credential = await startAuthentication({ optionsJSON: begin.options.publicKey });
      await authApi.passkeyLoginFinish(email, begin.flow, credential);
      await refresh();
      nav(next, { replace: true });
    } catch (e: any) {
      if (e?.name === "NotAllowedError") {
        setErr(t.auth.passkey.cancelledOrFailed);
      } else {
        setErr(String(e?.message || e));
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <Field label={t.auth.passkey.email}>
        <input
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          autoComplete="username webauthn"
          placeholder={t.auth.passkey.emailPlaceholder}
          className={inputCls}
        />
      </Field>
      {err && <div className="text-sm text-red-400">{err}</div>}
      <button
        type="submit"
        disabled={busy}
        className="w-full rounded-lg bg-brand-600 px-4 py-3 text-sm font-medium text-brand-ink hover:bg-brand-500 disabled:bg-neutral-700 disabled:text-neutral-500 inline-flex items-center justify-center gap-2"
      >
        <Icon name="fingerprint" />
        {busy ? t.auth.passkey.submitBusy : t.auth.passkey.submit}
      </button>
      <p className="text-xs text-neutral-500 text-center">
        {t.auth.passkey.hint}
      </p>
    </form>
  );
}
