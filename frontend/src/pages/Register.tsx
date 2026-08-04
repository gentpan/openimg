import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../AuthContext";
import { AuthShell, Field, inputCls } from "./Login";
import OAuthButtons from "./OAuthButtons";

export default function RegisterPage() {
  const { register } = useAuth();
  const nav = useNavigate();
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      await register(email, password, name || undefined);
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
        <Field label="昵称（可选）">
          <input value={name} onChange={(e) => setName(e.target.value)} className={inputCls} autoComplete="nickname" />
        </Field>
        <Field label="密码（至少 8 位）">
          <input type="password" required minLength={8} value={password} onChange={(e) => setPassword(e.target.value)} className={inputCls} autoComplete="new-password" />
        </Field>
        {err && <div className="text-sm text-red-400">{err}</div>}
        <button type="submit" disabled={busy} className="w-full rounded-lg bg-violet-600 px-4 py-3 text-sm font-medium text-white hover:bg-violet-500 disabled:bg-neutral-700">
          {busy ? "注册中…" : "创建账号"}
        </button>
      </form>
      <div className="mt-6">
        <OAuthButtons />
      </div>
      <div className="mt-5 text-sm text-neutral-500 text-center">
        已有账号？{" "}
        <Link to="/login" className="text-violet-400 hover:underline">
          登录
        </Link>
      </div>
    </AuthShell>
  );
}
