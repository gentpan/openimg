import { useState } from "react";
import { useAuth } from "../AuthContext";
import { userApi } from "../api";
import { useToast } from "../ToastContext";

const WIDTH_PRESETS = [
  { px: 0, label: "保持原尺寸" },
  { px: 3840, label: "4K · 3840" },
  { px: 2560, label: "2K · 2560" },
  { px: 1920, label: "1080p · 1920" },
  { px: 1280, label: "720p · 1280" },
];

const VARIANTS = [
  { key: "webp", label: "WebP", desc: "比原图小 25–35%，浏览器全面支持" },
  { key: "avif", label: "AVIF", desc: "比 WebP 再小 20–30%，编码慢，后台异步生成" },
  { key: "none", label: "不生成", desc: "只保留主图，最省空间" },
];

/**
 * Per-user upload settings.
 *
 * Two modes, and the difference is not cosmetic: optimised mode re-encodes and
 * strips metadata; original mode stores the exact bytes, EXIF and GPS included.
 * The consequences of the latter are spelled out rather than buried, because
 * "my photos leaked where I live" is not a recoverable mistake.
 */
export default function ConvertSettings() {
  const { user, refresh } = useAuth();
  const [busy, setBusy] = useState<string | null>(null);
  const toast = useToast();

  if (!user) return null;
  const original = user.upload_mode === "original";

  async function save(key: string, patch: Record<string, unknown>) {
    setBusy(key);
    try {
      await userApi.updatePreferences(patch);
      await refresh();
      toast.success("已保存", "对之后上传的图片生效，已上传的不受影响");
    } catch (e) {
      toast.error("保存失败", e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div>
      {/* Mode */}
      <div className="pb-3 mb-3 border-b border-neutral-800/60">
        <div className="text-xs text-neutral-200 mb-2">上传模式</div>
        <div className="grid sm:grid-cols-2 gap-2">
          <ModeCard
            active={!original}
            busy={busy === "mode"}
            onClick={() => save("mode", { upload_mode: "optimized" })}
            title="优化模式"
            badge="推荐"
            lines={["重新编码，剥离 EXIF 与 GPS", "可限制最大宽度", "体积通常只剩 10–30%"]}
          />
          <ModeCard
            active={original}
            busy={busy === "mode"}
            onClick={() => save("mode", { upload_mode: "original" })}
            title="原图模式"
            lines={["字节不变，保留全部 EXIF 与拍摄地点", "不压缩、不缩放", "占用空间显著更大"]}
            danger
          />
        </div>

        {original && (
          <div className="mt-2 rounded-lg border border-amber-500/30 bg-amber-950/20 px-3 py-2 text-[10px] text-amber-200/90 leading-relaxed">
            <i className="fa-solid fa-triangle-exclamation mr-1" />
            原图会连同 <b>EXIF 与 GPS 定位</b>一起公开 —— 任何拿到链接的人都能读出拍摄地点、时间和设备。
            分享手机拍摄的照片前请确认这是你想要的。
            <br />
            为安全起见，上传仍会校验真实格式（拒绝 SVG 与非图片），并强制以图片类型返回。
          </div>
        )}
      </div>

      {/* Width — meaningless in original mode */}
      <div className={`pb-3 mb-3 border-b border-neutral-800/60 ${original ? "opacity-40" : ""}`}>
        <div className="text-xs text-neutral-200">最大宽度</div>
        <div className="text-[10px] text-neutral-600 mt-0.5 mb-2 leading-relaxed">
          {original ? "原图模式下不缩放" : "超过该宽度的图片会等比缩小后再存储，小图不会被放大"}
        </div>
        <div className="flex flex-wrap gap-1.5">
          {WIDTH_PRESETS.map((w) => (
            <button
              key={w.px}
              disabled={busy === "width" || original}
              onClick={() => save("width", { max_image_width: w.px })}
              className={`inline-flex h-8 items-center justify-center px-3 rounded-lg text-xs transition disabled:cursor-not-allowed ${
                user.max_image_width === w.px
                  ? "bg-brand-600/20 text-brand-300 border border-brand-500/30"
                  : "bg-neutral-800 text-neutral-400 hover:text-neutral-100 border border-transparent"
              }`}
            >
              {w.label}
            </button>
          ))}
        </div>
      </div>

      {/* Single derivative */}
      <div>
        <div className="text-xs text-neutral-200">附加格式</div>
        <div className="text-[10px] text-neutral-600 mt-0.5 mb-2 leading-relaxed">
          只能选一种 —— 支持 AVIF 的浏览器必然也支持 WebP，两个都存等于为同一个回退付两次空间。
        </div>
        <div className="space-y-1.5">
          {VARIANTS.map((v) => (
            <button
              key={v.key}
              disabled={busy === "variant"}
              onClick={() => save("variant", { variant_format: v.key })}
              className={`w-full flex items-start gap-2.5 rounded-lg border px-3 py-2 text-left transition disabled:opacity-50 ${
                user.variant_format === v.key
                  ? "border-brand-500/40 bg-brand-950/20"
                  : "border-neutral-800 bg-neutral-950/40 hover:border-neutral-700"
              }`}
            >
              <span
                className={`mt-0.5 flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full border ${
                  user.variant_format === v.key ? "border-brand-500" : "border-neutral-600"
                }`}
              >
                {user.variant_format === v.key && <span className="h-1.5 w-1.5 rounded-full bg-brand-400" />}
              </span>
              <span className="min-w-0">
                <span className="block text-xs text-neutral-200">{v.label}</span>
                <span className="block text-[10px] text-neutral-600 mt-0.5">{v.desc}</span>
              </span>
            </button>
          ))}
        </div>
      </div>

      <div className="mt-3 rounded-lg bg-neutral-950/60 border border-neutral-800 px-3 py-2 text-[10px] text-neutral-500 leading-relaxed">
        <i className="fa-solid fa-circle-info mr-1 text-neutral-600" />
        上传时只生成 200px 网格缩略图，两种模式下都会生成 —— 图库列表用的就是它，
        直接渲染原图会让页面变得无法使用。600px 与 1200px 改为在图片详情里点击对应尺寸时才生成，
        不点就不占空间。你复制到外部的链接始终指向主图。
      </div>

    </div>
  );
}

function ModeCard({
  active,
  busy,
  onClick,
  title,
  badge,
  lines,
  danger,
}: {
  active: boolean;
  busy: boolean;
  onClick: () => void;
  title: string;
  badge?: string;
  lines: string[];
  danger?: boolean;
}) {
  return (
    <button
      onClick={onClick}
      disabled={busy}
      className={`rounded-xl border px-3 py-2.5 text-left transition disabled:opacity-50 ${
        active
          ? danger
            ? "border-amber-500/40 bg-amber-950/15"
            : "border-brand-500/40 bg-brand-950/20"
          : "border-neutral-800 bg-neutral-950/40 hover:border-neutral-700"
      }`}
    >
      <div className="flex items-center gap-1.5 mb-1.5">
        <span className="text-xs text-neutral-100">{title}</span>
        {badge && (
          <span className="rounded-full bg-brand-900/50 px-1.5 py-0.5 text-[9px] text-brand-300">{badge}</span>
        )}
        {active && <i className="fa-solid fa-check ml-auto text-[10px] text-brand-400" />}
      </div>
      <ul className="space-y-0.5">
        {lines.map((l) => (
          <li key={l} className="text-[10px] text-neutral-600 leading-relaxed">
            · {l}
          </li>
        ))}
      </ul>
    </button>
  );
}
