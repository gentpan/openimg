import Icon from "../Icon";
import { useEffect, useState } from "react";
import { formatBytes, storageApi } from "../api";
import type { ProfileInput, StorageProfile } from "../types";
import { RingSpinner } from "./Spinner";
import { useLang } from "../LangContext";
import type { Dict } from "../i18n";
import { useDialog } from "../DialogContext";

const EMPTY: ProfileInput = {
  name: "",
  endpoint: "",
  region: "auto",
  bucket: "",
  key_prefix: "",
  access_key: "",
  secret_key: "",
  public_base_url: "",
};

// Shown under the endpoint field so the user can sanity-check what we guessed.
// t comes in as a parameter: this is a module helper, not a component, and a
// hook inside it would silently depend on being called mid-render.
function describeEndpoint(t: Dict, endpoint: string): string {
  const e = endpoint.toLowerCase();
  if (!e) return "";
  if (e.includes("r2.cloudflarestorage.com")) return t.storageProfiles.endpoint.r2;
  if (e.includes("amazonaws.com")) return t.storageProfiles.endpoint.s3;
  if (e.includes("backblazeb2.com")) return t.storageProfiles.endpoint.b2;
  if (e.includes("digitaloceanspaces.com")) return t.storageProfiles.endpoint.spaces;
  if (e.includes("aliyuncs.com")) return t.storageProfiles.endpoint.oss;
  if (e.includes("myqcloud.com")) return t.storageProfiles.endpoint.cos;
  return t.storageProfiles.endpoint.custom;
}

/**
 * Bring-your-own-storage management. Every save round-trips through a live
 * write/read/delete probe on the server before anything is persisted — a
 * bucket that can't be written to is worse than no bucket, because uploads
 * would fail only after the user believes they're set up.
 */
/** "https://files.openimgcdn.com/" → "files.openimgcdn.com" */
function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return "";
  }
}

export default function StorageProfiles() {
  const dialog = useDialog();
  const { t } = useLang();
  const [profiles, setProfiles] = useState<StorageProfile[]>([]);
  const [editing, setEditing] = useState<StorageProfile | "new" | null>(null);
  const [form, setForm] = useState<ProfileInput>(EMPTY);
  const [busy, setBusy] = useState<string | null>(null);
  const [msg, setMsg] = useState<{ kind: "ok" | "err"; text: string } | null>(null);

  async function load() {
    setProfiles(await storageApi.list());
  }
  useEffect(() => {
    load().catch(() => {});
  }, []);

  function startEdit(p: StorageProfile | "new") {
    setMsg(null);
    setEditing(p);
    if (p === "new") {
      setForm(EMPTY);
    } else {
      setForm({
        name: p.name,
        endpoint: p.endpoint,
        region: p.region,
        bucket: p.bucket,
        key_prefix: p.key_prefix,
        access_key: "",
        secret_key: "",
        public_base_url: p.public_base_url,
      });
    }
  }

  async function submit(testOnly: boolean) {
    setBusy(testOnly ? "test" : "save");
    setMsg(null);
    try {
      const payload = { ...form, test_only: testOnly };
      if (editing === "new") await storageApi.create(payload);
      else if (editing) await storageApi.update(editing.id, payload);
      if (testOnly) {
        setMsg({ kind: "ok", text: t.storageProfiles.testPassed });
      } else {
        setMsg({ kind: "ok", text: t.common.saved });
        setEditing(null);
        await load();
      }
    } catch (e) {
      setMsg({ kind: "err", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(null);
    }
  }

  async function act(id: string, fn: () => Promise<unknown>, label: string) {
    setBusy(id);
    setMsg(null);
    try {
      await fn();
      await load();
      setMsg({ kind: "ok", text: label });
    } catch (e) {
      setMsg({ kind: "err", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(null);
    }
  }

  return (
    <div>
      {msg && (
        <div
          className={`mb-3 rounded-lg px-3 py-2 text-xs ${
            msg.kind === "ok" ? "bg-teal-950/30 text-teal-300" : "bg-red-950/30 text-red-300"
          }`}
        >
          {msg.text}
        </div>
      )}

      <div className="space-y-2 mb-3">
        {profiles.map((p) => (
          <div key={p.id} className="rounded-xl border border-neutral-800 bg-neutral-950/40 p-3">
            <div className="flex items-start gap-3">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-xs text-neutral-100">{p.name}</span>
                  {p.is_default && (
                    <span className="rounded-full bg-brand-900/50 px-1.5 py-0.5 text-[9px] text-brand-300">
                      {t.storageProfiles.badge.default}
                    </span>
                  )}
                  {p.is_platform && (
                    <span className="rounded-full bg-neutral-800 px-1.5 py-0.5 text-[9px] text-neutral-400">
                      {t.storageProfiles.badge.platform}
                    </span>
                  )}
                  {p.backup_of_id && (
                    <span className="rounded-full bg-amber-900/40 px-1.5 py-0.5 text-[9px] text-amber-300">
                      {t.storageProfiles.badge.backup}
                    </span>
                  )}
                  {p.status !== "active" && (
                    <span className="rounded-full bg-red-900/50 px-1.5 py-0.5 text-[9px] text-red-300">
                      {t.storageProfiles.badge.unavailable}
                    </span>
                  )}
                </div>
                {/* The platform pool's bucket, endpoint and access key belong
                    to us, not to the user: nothing they can act on, and an
                    endpoint of 127.0.0.1 reads as misconfigured. What they can
                    act on is the host their images are served from. Their own
                    buckets show the full config — that is the thing they came
                    to this page to check. */}
                <div className="mt-1 text-[10px] text-neutral-600 truncate">
                  {p.is_platform ? (
                    <>
                      <Icon name="globe" className="mr-1 text-neutral-700"  />
                      {hostOf(p.public_base_url) || t.storageProfiles.platformHosted}
                    </>
                  ) : (
                    <>
                      {p.bucket}
                      {p.endpoint && ` · ${p.endpoint}`}
                    </>
                  )}
                </div>
                <div className="text-[10px] text-neutral-600">
                  {t.common.countAndSize(p.image_count, formatBytes(p.stored_bytes, 0))}
                  {!p.is_platform && p.access_key_mask && ` · ${p.access_key_mask}`}
                </div>
                {p.last_error && <div className="mt-1 text-[10px] text-red-400">{p.last_error}</div>}
              </div>

              <div className="flex flex-col gap-1 shrink-0 text-[10px]">
                {!p.is_default && !p.backup_of_id && (
                  <button
                    onClick={() => act(p.id, () => storageApi.setDefault(p.id), t.storageProfiles.setDefaultDone)}
                    disabled={!!busy}
                    className="rounded-md bg-neutral-800 px-2 py-1 text-neutral-300 hover:bg-neutral-700 transition"
                  >
                    {t.storageProfiles.setDefault}
                  </button>
                )}
                {!p.is_platform && (
                  <>
                    <button
                      onClick={() => act(p.id, () => storageApi.test(p.id), t.storageProfiles.connectionOk)}
                      disabled={!!busy}
                      className="rounded-md bg-neutral-800 px-2 py-1 text-neutral-300 hover:bg-neutral-700 transition"
                    >
                      {t.storageProfiles.test}
                    </button>
                    <button
                      onClick={() => startEdit(p)}
                      disabled={!!busy}
                      className="rounded-md bg-neutral-800 px-2 py-1 text-neutral-300 hover:bg-neutral-700 transition"
                    >
                      {t.common.edit}
                    </button>
                    <button
                      onClick={async () => {
                        const ok = await dialog.confirm({
                          title: t.storageProfiles.deleteConfirm(p.name),
                          danger: true,
                          confirmLabel: t.common.delete,
                        });
                        if (ok) act(p.id, () => storageApi.remove(p.id), t.storageProfiles.deleted);
                      }}
                      disabled={!!busy}
                      className="rounded-md bg-neutral-800 px-2 py-1 text-red-400 hover:bg-neutral-700 transition"
                    >
                      {t.common.delete}
                    </button>
                  </>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>

      {editing ? (
        <div className="rounded-xl border border-brand-500/30 bg-neutral-950/60 p-4">
          <div className="text-xs text-neutral-200 mb-3">
            {editing === "new" ? t.storageProfiles.addTitle : t.storageProfiles.editTitle(editing.name)}
          </div>
          <div className="grid sm:grid-cols-2 gap-2.5">
            <Field label={t.storageProfiles.field.name} value={form.name} onChange={(v) => setForm({ ...form, name: v })} placeholder={t.storageProfiles.field.namePlaceholder} />
            <Field label="Bucket" value={form.bucket} onChange={(v) => setForm({ ...form, bucket: v })} />
            <div className="sm:col-span-2">
              <Field
                label="Endpoint"
                value={form.endpoint}
                onChange={(v) => setForm({ ...form, endpoint: v })}
                placeholder={t.storageProfiles.field.endpointPlaceholder}
              />
              {form.endpoint && (
                <div className="mt-1 text-[10px] text-brand-400/80">
                  <Icon name="wand-magic-sparkles" className="mr-1"  />
                  {describeEndpoint(t, form.endpoint)}
                </div>
              )}
            </div>
            <Field label="Region" value={form.region} onChange={(v) => setForm({ ...form, region: v })} />
            <Field
              label={t.storageProfiles.field.keyPrefix}
              value={form.key_prefix}
              onChange={(v) => setForm({ ...form, key_prefix: v })}
              placeholder="img/"
            />
            <Field
              label="Access Key"
              value={form.access_key}
              onChange={(v) => setForm({ ...form, access_key: v })}
              placeholder={editing === "new" ? "" : t.storageProfiles.field.unchangedPlaceholder}
            />
            <Field
              label="Secret Key"
              value={form.secret_key}
              onChange={(v) => setForm({ ...form, secret_key: v })}
              type="password"
              placeholder={editing === "new" ? "" : t.storageProfiles.field.unchangedPlaceholder}
            />
            <div className="sm:col-span-2">
              <Field
                label={t.storageProfiles.field.publicBaseUrl}
                value={form.public_base_url}
                onChange={(v) => setForm({ ...form, public_base_url: v })}
                placeholder="https://img.example.com"
              />
            </div>
          </div>

          <div className="mt-3 rounded-lg bg-amber-950/20 border border-amber-500/20 px-3 py-2 text-[10px] text-amber-200/90">
            <Icon name="shield-halved" className="mr-1"  />
            {t.storageProfiles.securityNote}
          </div>

          <div className="mt-3 flex items-center gap-2">
            <button
              onClick={() => submit(true)}
              disabled={!!busy}
              className="inline-flex h-8 items-center justify-center rounded-lg bg-neutral-800 px-3 text-xs text-neutral-200 hover:bg-neutral-700 disabled:opacity-50 transition"
            >
              {busy === "test" ? <RingSpinner className="h-3.5 w-3.5 inline-block align-[-2px]" /> : t.storageProfiles.testConnection}
            </button>
            <button
              onClick={() => submit(false)}
              disabled={!!busy}
              className="inline-flex h-8 items-center justify-center rounded-lg bg-brand-600 px-3 text-xs font-medium text-brand-ink hover:bg-brand-500 disabled:opacity-50 transition"
            >
              {busy === "save" ? <RingSpinner className="h-3.5 w-3.5 inline-block align-[-2px]" /> : t.common.save}
            </button>
            <button
              onClick={() => {
                setEditing(null);
                setMsg(null);
              }}
              className="text-xs text-neutral-500 hover:text-neutral-300"
            >
              {t.common.cancel}
            </button>
          </div>
        </div>
      ) : (
        <button
          onClick={() => startEdit("new")}
          className="inline-flex h-8 items-center justify-center rounded-lg bg-neutral-800 px-3 text-xs text-neutral-200 hover:bg-neutral-700 transition"
        >
          <Icon name="plus" className="mr-1.5"  />
          {t.storageProfiles.addTitle}
        </button>
      )}
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  placeholder,
  type = "text",
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  type?: string;
}) {
  return (
    <div>
      <label className="block text-[10px] text-neutral-600 mb-1">{label}</label>
      <input
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg bg-neutral-900 border border-neutral-800 h-8 px-2.5 text-xs outline-none focus:border-brand-500 placeholder-faint"
      />
    </div>
  );
}
