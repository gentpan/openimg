import { useEffect, useState } from "react";
import { adminApi } from "../../api";
import { RingSpinner } from "../../components/Spinner";

/**
 * Site-wide upload behaviour. These are operator decisions rather than user
 * preferences because they spend the platform's storage, not the uploader's
 * intent — a user picking "keep everything" for themselves would quietly
 * double what the pool has to hold.
 */
export default function UploadSettings() {
  const [keepOriginal, setKeepOriginal] = useState(false);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ kind: "ok" | "err"; text: string } | null>(null);

  useEffect(() => {
    adminApi
      .uploadSettings()
      .then((s) => setKeepOriginal(s.keep_original))
      .catch((e) => setMsg({ kind: "err", text: String(e.message || e) }))
      .finally(() => setLoading(false));
  }, []);

  async function save(next: boolean) {
    setBusy(true);
    setMsg(null);
    const prev = keepOriginal;
    setKeepOriginal(next);
    try {
      await adminApi.saveUploadSettings({ keep_original: next });
      setMsg({ kind: "ok", text: next ? "已开启，之后上传的图片会同时保留原图" : "已关闭" });
    } catch (e) {
      setKeepOriginal(prev);
      setMsg({ kind: "err", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(false);
    }
  }

  if (loading) return <div className="text-xs text-neutral-600">加载中…</div>;

  return (
    <div className="space-y-3">
      {msg && (
        <div
          className={`rounded-xl px-4 py-2.5 text-xs ${
            msg.kind === "ok"
              ? "border border-emerald-500/30 bg-emerald-950/20 text-emerald-200"
              : "border border-red-500/30 bg-red-950/20 text-red-200"
          }`}
        >
          {msg.text}
        </div>
      )}

      <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-4">
        <div className="flex items-start gap-3">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <span className="text-sm text-neutral-100">上传保留原图</span>
              {busy && <RingSpinner className="h-3 w-3 text-violet-400" />}
            </div>
            <p className="mt-1.5 text-[11px] leading-relaxed text-neutral-500">
              开启后，压缩转换的同时额外保存一份未经修改的上传文件，用户可以在图片详情里下载原图。
              原图带完整 EXIF，占用空间约翻倍，只对开启之后的新上传生效。
            </p>
          </div>
          <Toggle checked={keepOriginal} disabled={busy} onChange={save} />
        </div>
      </div>

      <ImageDomain />

      <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-4">
        <div className="text-sm text-neutral-100 mb-2">缩略图策略</div>
        <p className="text-[11px] leading-relaxed text-neutral-500">
          上传时只生成 200px 网格缩略图（约 8 KB），图库列表用它。600px 与 1200px
          在用户点击对应尺寸时才生成——大多数图片一辈子只会被看一种尺寸，提前全生成会白白多占 5–23% 的空间。
        </p>
      </div>
    </div>
  );
}

function Toggle({
  checked,
  disabled,
  onChange,
}: {
  checked: boolean;
  disabled?: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <button
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={`relative shrink-0 h-6 w-11 rounded-full transition disabled:opacity-50 ${
        checked ? "bg-violet-600" : "bg-neutral-700"
      }`}
    >
      <span
        className="absolute top-0.5 h-5 w-5 rounded-full bg-white shadow-[0_1px_3px_rgb(0_0_0/0.3)] transition-all duration-200"
        style={{ left: checked ? "1.375rem" : "0.125rem" }}
      />
    </button>
  );
}

/**
 * The domain browsers fetch images from.
 *
 * Stored on the platform storage profile, not read from the environment: the
 * profile is written once at first boot, so after that S3_PUBLIC_URL_BASE has
 * no effect and this is the only way to move to a different image domain.
 *
 * Deliberately separate from the site's own address. Serving images from their
 * own domain keeps session cookies off image requests and lets the CDN in
 * front of the bucket be swapped without touching the app.
 */
function ImageDomain() {
  const [info, setInfo] = useState<Awaited<ReturnType<typeof adminApi.platformStorage>> | null>(null);
  const [value, setValue] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ kind: "ok" | "err"; text: string } | null>(null);

  async function load() {
    const s = await adminApi.platformStorage();
    setInfo(s);
    setValue(s.public_base_url);
  }
  useEffect(() => {
    load().catch((e) => setMsg({ kind: "err", text: String(e.message || e) }));
  }, []);

  async function save() {
    setBusy(true);
    setMsg(null);
    try {
      const r = await adminApi.savePlatformStorage(value.trim());
      await load();
      setMsg({
        kind: "ok",
        text: `已切换，${r.affected} 张图片的链接立即指向新域名（对象未移动）`,
      });
    } catch (e) {
      setMsg({ kind: "err", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(false);
    }
  }

  if (!info) return null;

  const dirty = value.trim().replace(/\/+$/, "") !== info.public_base_url;

  return (
    <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-4">
      <div className="flex items-center gap-2 mb-1">
        <span className="text-sm text-neutral-100">图片访问域名</span>
        {busy && <RingSpinner className="h-3 w-3 text-violet-400" />}
      </div>

      {!info.configured ? (
        <p className="text-[11px] leading-relaxed text-neutral-500">
          尚未配置平台存储（当前为本地磁盘开发模式），图片链接暂时使用站点地址{" "}
          <code className="text-neutral-400">{info.site_base_url}</code>。
        </p>
      ) : (
        <>
          <p className="text-[11px] leading-relaxed text-neutral-500 mb-3">
            所有图片外链的前缀，与站点域名相互独立。改这里不会移动任何文件——链接是读取时拼出来的，
            保存后 {info.image_count} 张已有图片会立刻改用新域名。
            <span className="block mt-1 text-amber-300">
              前提：新域名必须已经指向同一个存储桶（{info.bucket}），否则所有图片会立即 404。
            </span>
          </p>

          <div className="flex flex-wrap items-center gap-1.5">
            <input
              value={value}
              onChange={(e) => setValue(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && dirty && save()}
              placeholder="https://imgla.com"
              className="flex-1 min-w-[14rem] rounded-lg bg-neutral-950 border border-neutral-800 px-2.5 py-1.5 text-xs font-mono outline-none focus:border-violet-500 placeholder-faint"
            />
            <button
              onClick={save}
              disabled={busy || !dirty || !value.trim()}
              className="rounded-lg bg-violet-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-violet-500 disabled:bg-neutral-800 disabled:text-neutral-500 transition"
            >
              保存
            </button>
          </div>

          <div className="mt-2 text-[10px] text-faint break-all">
            当前链接示例：{info.sample_url}
          </div>
        </>
      )}

      {msg && (
        <div
          className={`mt-2 text-[11px] ${msg.kind === "ok" ? "text-emerald-300" : "text-red-400"}`}
        >
          {msg.text}
        </div>
      )}
    </div>
  );
}
