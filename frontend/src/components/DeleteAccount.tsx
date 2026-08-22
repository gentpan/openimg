import Icon from "../Icon";
import { useState } from "react";
import { useAuth } from "../AuthContext";
import { userApi } from "../api";
import { RingSpinner } from "./Spinner";
import { useLang } from "../LangContext";
import { useDialog } from "../DialogContext";

/**
 * Account deletion. Typing the account's own email is the confirmation — a
 * checkbox is too easy to click through for something irreversible, and a
 * password prompt would lock out OAuth-only and passkey-only accounts.
 */
export default function DeleteAccount() {
  const dialog = useDialog();
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
      await dialog.alert({ title: res.deleted_images ? t.deleteAccount.doneWithImages(res.deleted_images) : t.deleteAccount.done });
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
        <Icon name="trash-can" className="mr-1.5"  />
        {t.deleteAccount.button}
      </button>
    );
  }

  return (
    <div className="rounded-xl border border-red-500/30 bg-red-950/15 p-4">
      <div className="text-sm text-red-200 mb-2">
        <Icon name="triangle-exclamation" className="mr-1.5"  />
        {t.deleteAccount.warningTitle}
      </div>
      <ul className="text-[11px] text-neutral-400 space-y-1 mb-3 leading-relaxed">
        <li>· {t.deleteAccount.warning.images}</li>
        <li>· {t.deleteAccount.warning.rewards}</li>
        <li>· {t.deleteAccount.warning.tokens}</li>
        <li>· {t.deleteAccount.warning.reregister}</li>
      </ul>

      <label className="block text-[10px] text-neutral-500 mb-1">
        {t.deleteAccount.typeEmailToConfirm(user.email)}
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
