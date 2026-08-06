import { useState } from "react";
import { useAuth } from "../AuthContext";
import { userApi } from "../api";
import { RingSpinner } from "./Spinner";
import { useLang } from "../LangContext";

/**
 * Account deletion. Typing the account's own email is the confirmation — a
 * checkbox is too easy to click through for something irreversible, and a
 * password prompt would lock out OAuth-only and passkey-only accounts.
 */
export default function DeleteAccount() {
  const { t } = useLang();
  const { user } = useAuth();
  const [open, setOpen] = useState(false);
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  if (!user) return null;
  const matches = confirm.trim().toLowerCase() === user.email.toLowerCase();

  async function remove() {
    setBusy(true);
    setErr(null);
    try {
      const res = await userApi.deleteAccount(confirm.trim());
      alert(`账号已删除${res.deleted_images ? `，同时删除了 ${res.deleted_images} 张图片` : ""}`);
      window.location.href = "/";
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
      setBusy(false);
    }
  }

  if (!open) {
    return (
      <button
        onClick={() => setOpen(true)}
        className="inline-flex h-8 items-center justify-center rounded-lg bg-red-600 px-3 text-xs font-medium text-white hover:bg-red-700 transition"
      >
        <i className="fa-solid fa-trash-can mr-1.5" />
        {t.deleteAccount.button}
      </button>
    );
  }

  return (
    <div className="rounded-xl border border-red-500/30 bg-red-950/15 p-4">
      <div className="text-sm text-red-200 mb-2">
        <i className="fa-solid fa-triangle-exclamation mr-1.5" />
        {t.deleteAccount.warningTitle}
      </div>
      <ul className="text-[11px] text-neutral-400 space-y-1 mb-3 leading-relaxed">
        <li>· 你上传的<b className="text-red-300">{t.deleteAccount.warning.imagesEmphasis}</b>，所有外链立即失效</li>
        <li>· 已获得的空间、签到记录、邀请奖励全部清空</li>
        <li>· API Token 立即失效，绑定的自有存储配置会被移除（不会删除你自己桶里的文件）</li>
        <li>· 该邮箱可以重新注册，但数据无法找回</li>
      </ul>

      <label className="block text-[10px] text-neutral-500 mb-1">
        请输入 <span className="font-mono text-neutral-300">{user.email}</span> 以确认
      </label>
      <input
        value={confirm}
        onChange={(e) => setConfirm(e.target.value)}
        placeholder={user.email}
        autoComplete="off"
        className="w-full rounded-lg bg-neutral-900 border border-neutral-800 h-8 px-2.5 text-xs outline-none focus:border-red-500 placeholder-faint mb-3"
      />

      {err && <div className="mb-2 text-[11px] text-red-400">{err}</div>}

      <div className="flex items-center gap-2">
        <button
          onClick={remove}
          disabled={!matches || busy}
          className="inline-flex h-8 items-center justify-center rounded-lg bg-red-600 px-3 text-xs font-medium text-white hover:bg-red-700 disabled:bg-neutral-800 disabled:text-neutral-600 transition"
        >
          {busy ? <RingSpinner className="h-3.5 w-3.5 inline-block align-[-2px]" /> : t.deleteAccount.confirmButton}
        </button>
        <button
          onClick={() => {
            setOpen(false);
            setConfirm("");
            setErr(null);
          }}
          className="text-xs text-neutral-500 hover:text-neutral-300"
        >
          {t.common.cancel}
        </button>
      </div>
    </div>
  );
}
