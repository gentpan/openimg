import { useEffect, useState } from "react";
import { formatBytes, storageApi } from "../api";
import type { ProfileInput, StorageProfile } from "../types";
import { RingSpinner } from "./Spinner";

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
function describeEndpoint(endpoint: string): string {
  const e = endpoint.toLowerCase();
  if (!e) return "";
  if (e.includes("r2.cloudflarestorage.com")) return "识别为 Cloudflare R2 · 虚拟主机寻址";
  if (e.includes("amazonaws.com")) return "识别为 AWS S3 · 虚拟主机寻址";
  if (e.includes("backblazeb2.com")) return "识别为 Backblaze B2 · 虚拟主机寻址";
  if (e.includes("digitaloceanspaces.com")) return "识别为 DigitalOcean Spaces · 虚拟主机寻址";
  if (e.includes("aliyuncs.com")) return "识别为阿里云 OSS · 虚拟主机寻址";
  if (e.includes("myqcloud.com")) return "识别为腾讯云 COS · 虚拟主机寻址";
  return "按自建 S3（MinIO 等）处理 · 路径寻址";
}

/**
 * Bring-your-own-storage management. Every save round-trips through a live
 * write/read/delete probe on the server before anything is persisted — a
 * bucket that can't be written to is worse than no bucket, because uploads
 * would fail only after the user believes they're set up.
 */
/** "https://cdn.imgla.com/" → "cdn.imgla.com" */
function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return "";
  }
}

export default function StorageProfiles() {
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
        setMsg({ kind: "ok", text: "连接测试通过，可以保存了" });
      } else {
        setMsg({ kind: "ok", text: "已保存" });
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
            msg.kind === "ok" ? "bg-emerald-950/30 text-emerald-300" : "bg-red-950/30 text-red-300"
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
                    <span className="rounded-full bg-violet-900/50 px-1.5 py-0.5 text-[9px] text-violet-300">
                      默认
                    </span>
                  )}
                  {p.is_platform && (
                    <span className="rounded-full bg-neutral-800 px-1.5 py-0.5 text-[9px] text-neutral-400">
                      平台
                    </span>
                  )}
                  {p.backup_of_id && (
                    <span className="rounded-full bg-amber-900/40 px-1.5 py-0.5 text-[9px] text-amber-300">
                      备份
                    </span>
                  )}
                  {p.status !== "active" && (
                    <span className="rounded-full bg-red-900/50 px-1.5 py-0.5 text-[9px] text-red-300">
                      不可用
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
                      <i className="fa-solid fa-globe mr-1 text-neutral-700" />
                      {hostOf(p.public_base_url) || "平台托管"}
                    </>
                  ) : (
                    <>
                      {p.bucket}
                      {p.endpoint && ` · ${p.endpoint}`}
                    </>
                  )}
                </div>
                <div className="text-[10px] text-neutral-600">
                  {p.image_count} 张 · {formatBytes(p.stored_bytes, 0)}
                  {!p.is_platform && p.access_key_mask && ` · ${p.access_key_mask}`}
                </div>
                {p.last_error && <div className="mt-1 text-[10px] text-red-400">{p.last_error}</div>}
              </div>

              <div className="flex flex-col gap-1 shrink-0 text-[10px]">
                {!p.is_default && !p.backup_of_id && (
                  <button
                    onClick={() => act(p.id, () => storageApi.setDefault(p.id), "已设为默认上传目标")}
                    disabled={!!busy}
                    className="rounded-md bg-neutral-800 px-2 py-1 text-neutral-300 hover:bg-neutral-700 transition"
                  >
                    设为默认
                  </button>
                )}
                {!p.is_platform && (
                  <>
                    <button
                      onClick={() => act(p.id, () => storageApi.test(p.id), "连接正常")}
                      disabled={!!busy}
                      className="rounded-md bg-neutral-800 px-2 py-1 text-neutral-300 hover:bg-neutral-700 transition"
                    >
                      测试
                    </button>
                    <button
                      onClick={() => startEdit(p)}
                      disabled={!!busy}
                      className="rounded-md bg-neutral-800 px-2 py-1 text-neutral-300 hover:bg-neutral-700 transition"
                    >
                      编辑
                    </button>
                    <button
                      onClick={() => {
                        if (confirm(`删除存储配置「${p.name}」？`))
                          act(p.id, () => storageApi.remove(p.id), "已删除");
                      }}
                      disabled={!!busy}
                      className="rounded-md bg-neutral-800 px-2 py-1 text-red-400 hover:bg-neutral-700 transition"
                    >
                      删除
                    </button>
                  </>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>

      {editing ? (
        <div className="rounded-xl border border-violet-500/30 bg-neutral-950/60 p-4">
          <div className="text-xs text-neutral-200 mb-3">
            {editing === "new" ? "添加自有存储" : `编辑 ${editing.name}`}
          </div>
          <div className="grid sm:grid-cols-2 gap-2.5">
            <Field label="名称" value={form.name} onChange={(v) => setForm({ ...form, name: v })} placeholder="我的存储" />
            <Field label="Bucket" value={form.bucket} onChange={(v) => setForm({ ...form, bucket: v })} />
            <div className="sm:col-span-2">
              <Field
                label="Endpoint"
                value={form.endpoint}
                onChange={(v) => setForm({ ...form, endpoint: v })}
                placeholder="https://<account>.r2.cloudflarestorage.com 或 https://s3.example.com"
              />
              {form.endpoint && (
                <div className="mt-1 text-[10px] text-violet-400/80">
                  <i className="fa-solid fa-wand-magic-sparkles mr-1" />
                  {describeEndpoint(form.endpoint)}
                </div>
              )}
            </div>
            <Field label="Region" value={form.region} onChange={(v) => setForm({ ...form, region: v })} />
            <Field
              label="Key 前缀（可选）"
              value={form.key_prefix}
              onChange={(v) => setForm({ ...form, key_prefix: v })}
              placeholder="img/"
            />
            <Field
              label="Access Key"
              value={form.access_key}
              onChange={(v) => setForm({ ...form, access_key: v })}
              placeholder={editing === "new" ? "" : "留空表示不修改"}
            />
            <Field
              label="Secret Key"
              value={form.secret_key}
              onChange={(v) => setForm({ ...form, secret_key: v })}
              type="password"
              placeholder={editing === "new" ? "" : "留空表示不修改"}
            />
            <div className="sm:col-span-2">
              <Field
                label="公开访问地址"
                value={form.public_base_url}
                onChange={(v) => setForm({ ...form, public_base_url: v })}
                placeholder="https://img.example.com"
              />
            </div>
          </div>

          <div className="mt-3 rounded-lg bg-amber-950/20 border border-amber-500/20 px-3 py-2 text-[10px] text-amber-200/90">
            <i className="fa-solid fa-shield-halved mr-1" />
            建议创建<b>仅限该 bucket、仅有对象读写权限</b>的令牌。密钥会以 AES-256-GCM 加密存储，页面上永不回显。
          </div>

          <div className="mt-3 flex items-center gap-2">
            <button
              onClick={() => submit(true)}
              disabled={!!busy}
              className="inline-flex h-8 items-center justify-center rounded-lg bg-neutral-800 px-3 text-xs text-neutral-200 hover:bg-neutral-700 disabled:opacity-50 transition"
            >
              {busy === "test" ? <RingSpinner className="h-3.5 w-3.5 inline-block align-[-2px]" /> : "测试连接"}
            </button>
            <button
              onClick={() => submit(false)}
              disabled={!!busy}
              className="inline-flex h-8 items-center justify-center rounded-lg bg-violet-600 px-3 text-xs font-medium text-white hover:bg-violet-500 disabled:opacity-50 transition"
            >
              {busy === "save" ? <RingSpinner className="h-3.5 w-3.5 inline-block align-[-2px]" /> : "保存"}
            </button>
            <button
              onClick={() => {
                setEditing(null);
                setMsg(null);
              }}
              className="text-xs text-neutral-500 hover:text-neutral-300"
            >
              取消
            </button>
          </div>
        </div>
      ) : (
        <button
          onClick={() => startEdit("new")}
          className="inline-flex h-8 items-center justify-center rounded-lg bg-neutral-800 px-3 text-xs text-neutral-200 hover:bg-neutral-700 transition"
        >
          <i className="fa-solid fa-plus mr-1.5" />
          添加自有存储
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
        className="w-full rounded-lg bg-neutral-900 border border-neutral-800 h-8 px-2.5 text-xs outline-none focus:border-violet-500 placeholder-faint"
      />
    </div>
  );
}
