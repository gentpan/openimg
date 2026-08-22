import Icon from "../Icon";
import { useState } from "react";
import { useAuth } from "../AuthContext";
import { userApi } from "../api";
import { useToast } from "../ToastContext";
import { useLang } from "../LangContext";
import type { Dict } from "../i18n";

// Takes the dictionary rather than closing over it: a module constant is
// evaluated once at import, and the language can change at any point after.
const widthPresets = (t: Dict) => [
  { px: 0, label: t.convertSettings.width.keepOriginal },
  { px: 3840, label: "4K · 3840" },
  { px: 2560, label: "2K · 2560" },
  { px: 1920, label: "1080p · 1920" },
  { px: 1280, label: "720p · 1280" },
];

const variants = (t: Dict) => [
  { key: "webp", label: "WebP", desc: t.convertSettings.variant.webpDesc },
  { key: "avif", label: "AVIF", desc: t.convertSettings.variant.avifDesc },
  { key: "none", label: t.convertSettings.variant.noneLabel, desc: t.convertSettings.variant.noneDesc },
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
  const { t } = useLang();
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
      toast.success(t.common.saved, t.convertSettings.savedDetail);
    } catch (e) {
      toast.error(t.common.saveFailed, e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div>
      {/* Mode */}
      <div className="pb-3 mb-3 border-b border-neutral-800/60">
        <div className="text-xs text-neutral-200 mb-2">{t.convertSettings.mode.title}</div>
        <div className="grid sm:grid-cols-2 gap-2">
          <ModeCard
            active={!original}
            busy={busy === "mode"}
            onClick={() => save("mode", { upload_mode: "optimized" })}
            title={t.convertSettings.mode.optimized}
            badge={t.convertSettings.badgeRecommended}
            lines={[t.convertSettings.mode.optimizedLine1, t.convertSettings.mode.optimizedLine2, t.convertSettings.mode.optimizedLine3]}
          />
          <ModeCard
            active={original}
            busy={busy === "mode"}
            onClick={() => save("mode", { upload_mode: "original" })}
            title={t.convertSettings.mode.original}
            lines={[t.convertSettings.mode.originalLine1, t.convertSettings.mode.originalLine2, t.convertSettings.mode.originalLine3]}
            danger
          />
        </div>

        {original && (
          <div className="mt-2 rounded-lg border border-amber-500/30 bg-amber-950/20 px-3 py-2 text-[10px] text-amber-200/90 leading-relaxed">
            <Icon name="triangle-exclamation" className="mr-1"  />
              {t.convertSettings.original.warning}
              <br />
              {t.convertSettings.original.warningSecurity}
          </div>
        )}
      </div>

      {/* Width — meaningless in original mode */}
      <div className={`pb-3 mb-3 border-b border-neutral-800/60 ${original ? "opacity-40" : ""}`}>
        <div className="text-xs text-neutral-200">{t.convertSettings.width.title}</div>
        <div className="text-[10px] text-neutral-600 mt-0.5 mb-2 leading-relaxed">
          {original ? t.convertSettings.width.disabledHint : t.convertSettings.width.hint}
        </div>
        <div className="flex flex-wrap gap-1.5">
          {widthPresets(t).map((w) => (
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

      {/* Conversion target — like width, meaningless in original mode */}
      <div className={original ? "opacity-40" : ""}>
        <div className="text-xs text-neutral-200">{t.convertSettings.variant.title}</div>
        <div className="text-[10px] text-neutral-600 mt-0.5 mb-2 leading-relaxed">
          {original ? t.convertSettings.variant.disabledHint : t.convertSettings.variant.hint}
        </div>
        <div className="space-y-1.5">
          {variants(t).map((v) => (
            <button
              key={v.key}
              disabled={busy === "variant" || original}
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

        {/* Thumbnail policy — applies in both modes, since thumbnails are
            generated either way. */}
        <div className="mt-3 pt-3 border-t border-neutral-800/60">
          <div className="text-xs text-neutral-200">{t.convertSettings.thumb.title}</div>
          <div className="text-[10px] text-neutral-600 mt-0.5 mb-2 leading-relaxed">
            {t.convertSettings.thumb.hint}
          </div>
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="text-[10px] text-neutral-600 mr-1">{t.convertSettings.thumb.widthTitle}</span>
            {[200, 400, 600, 800, 1000].map((w) => (
              <button
                key={w}
                disabled={busy === "thumbWidth"}
                onClick={() => save("thumbWidth", { thumb_width: w })}
                className={`inline-flex h-8 items-center justify-center px-3 rounded-lg text-xs transition disabled:cursor-not-allowed ${
                  user.thumb_width === w
                    ? "bg-brand-600/20 text-brand-300 border border-brand-500/30"
                    : "bg-neutral-800 text-neutral-400 hover:text-neutral-100 border border-transparent"
                }`}
              >
                {w}px
              </button>
            ))}
          </div>
          <div className="mt-2 flex flex-wrap items-center gap-1.5">
            <span className="text-[10px] text-neutral-600 mr-1">{t.convertSettings.thumb.formatTitle}</span>
            {(
              [
                ["webp", "WebP", t.convertSettings.thumb.formatWebp],
                ["avif", "AVIF", t.convertSettings.thumb.formatAvif],
                ["jpg", "JPEG", t.convertSettings.thumb.formatJpg],
              ] as const
            ).map(([key, label, desc]) => (
              <button
                key={key}
                disabled={busy === "thumbFormat"}
                title={desc}
                onClick={() => save("thumbFormat", { thumb_format: key })}
                className={`inline-flex h-8 items-center justify-center px-3 rounded-lg text-xs transition disabled:cursor-not-allowed ${
                  user.thumb_format === key
                    ? "bg-brand-600/20 text-brand-300 border border-brand-500/30"
                    : "bg-neutral-800 text-neutral-400 hover:text-neutral-100 border border-transparent"
                }`}
              >
                {label}
              </button>
            ))}
          </div>
        </div>

      <div className="mt-3 rounded-lg bg-neutral-950/60 border border-neutral-800 px-3 py-2 text-[10px] text-neutral-500 leading-relaxed">
        <Icon name="circle-info" className="mr-1 text-neutral-600"  />
          {t.convertSettings.thumbnailNote}
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
        {active && <Icon name="check" className="ml-auto text-[10px] text-brand-400"  />}
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
