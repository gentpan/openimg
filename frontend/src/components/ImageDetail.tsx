import { useState } from "react";
import { formatBytes, imageApi, reportApi } from "../api";
import Spinner, { RingSpinner } from "./Spinner";
import { useToast } from "../ToastContext";
import type { Image } from "../types";
import { useLang } from "../LangContext";

/**
 * Slide-over detail panel for one image: preview, metadata, every link format,
 * and the derivative URLs. Shared by the gallery and the upload page so the
 * two can't drift — this is where users actually copy links from.
 */
const SIZES = [200, 600, 1200] as const;

export default function DetailPanel({ img, onClose, onDeleted }: { img: Image; onClose: () => void; onDeleted: () => void }) {
  const { t, locale } = useLang();
  const [copied, setCopied] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [reporting, setReporting] = useState(false);
  // Variants are generated on demand now, so this panel owns a local copy of
  // the list that grows as the user asks for sizes.
  const [variants, setVariants] = useState<Record<string, string>>(img.variant_urls);
  // "orig-jpg" style name; the extension varies because it is whatever was
  // uploaded, so the key has to be looked up rather than hard-coded.
  const originalKey = img.variants.split(",").find((v) => v.startsWith("orig-"));
  const [making, setMaking] = useState<number | null>(null);
  const [genErr, setGenErr] = useState<string | null>(null);

  async function makeSize(w: (typeof SIZES)[number]) {
    setMaking(w);
    setGenErr(null);
    try {
      const res = await imageApi.makeVariant(img.id, w);
      setVariants((prev) => ({ ...prev, [res.variant]: res.url }));
    } catch (e) {
      setGenErr(e instanceof Error ? e.message : String(e));
    } finally {
      setMaking(null);
    }
  }

  async function copy(text: string, key: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(key);
      setTimeout(() => setCopied(null), 1500);
    } catch {}
  }

  async function remove() {
    if (!confirm(t.imageDetail.deleteConfirm)) return;
    setBusy(true);
    try {
      await imageApi.remove(img.id);
      onDeleted();
    } catch (e) {
      alert(e instanceof Error ? e.message : String(e));
      setBusy(false);
    }
  }

  // The preview must not be the original. This panel is ~400px wide and the
  // original can be several megabytes — one of the images on the live site is
  // a 4.2 MB PNG whose 6 KB thumbnail renders identically at this size.
  //
  // Ordered by fitness for that width, not by quality: w600 is the closest
  // match, the full-size re-encodes are correct but oversized, and w200 is the
  // last resort before falling back to the original.
  const previewSrc =
    variants.w600 ??
    variants.w1200 ??
    variants.avif ??
    variants.webp ??
    variants.w200 ??
    img.thumb_url ??
    img.url;

  const links = [
    ...(img.short_url ? [{ key: "short", label: t.common.shortLink, value: img.short_url }] : []),
    { key: "url", label: "URL", value: img.url },
    { key: "md", label: "Markdown", value: img.markdown },
    { key: "html", label: "HTML", value: img.html },
    { key: "bb", label: "BBCode", value: img.bbcode },
  ];

  if (reporting) {
    return <ReportDialog img={img} onClose={() => setReporting(false)} />;
  }

  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div className="relative w-full max-w-md bg-neutral-900 border-l border-neutral-800 flex flex-col overflow-y-auto">
        <div className="p-4 border-b border-neutral-800 flex items-center justify-between">
          <span className="text-sm font-medium text-neutral-100 truncate">{img.orig_name}</span>
          <button onClick={onClose} className="text-neutral-400 hover:text-neutral-100 shrink-0 ml-3">
            <i className="fa-solid fa-xmark" />
          </button>
        </div>

        <div className="p-4">
          <img
            src={previewSrc}
            alt={img.orig_name}
            loading="lazy"
            className="w-full rounded-xl mb-4 bg-neutral-950"
          />

          <dl className="space-y-1 text-[11px] mb-4">
            <Row label={t.imageDetail.dimensions} value={`${img.width} × ${img.height}`} />
            <Row label={t.imageDetail.originalSize} value={formatBytes(img.size_orig)} />
            <Row label={t.imageDetail.storedSize} value={formatBytes(img.size_stored)} />
            <Row label={t.imageDetail.format} value={img.ext.toUpperCase()} />
            <Row label={t.imageDetail.uploadedAt} value={new Date(img.created_at).toLocaleString(locale)} />
            {img.variants && (
              <Row
                label={t.imageDetail.generated}
                value={img.variants
                  .split(",")
                  .map((v) => (v.startsWith("orig-") ? t.imageDetail.originalVariant : v))
                  .join(" · ")}
              />
            )}
          </dl>

          <div className="space-y-2 mb-4">
            {links.map((l) => (
              <div key={l.key}>
                <div className="text-[10px] text-neutral-600 mb-1">{l.label}</div>
                <div className="flex items-center gap-1.5">
                  <input
                    readOnly
                    value={l.value}
                    onFocus={(e) => e.currentTarget.select()}
                    className="flex-1 min-w-0 rounded-md bg-neutral-950 border border-neutral-800 px-2 py-1.5 text-[11px] text-neutral-400 outline-none focus:border-brand-500"
                  />
                  <button
                    onClick={() => copy(l.value, l.key)}
                    className={`shrink-0 inline-flex items-center justify-center w-7 h-7 rounded-md text-[10px] transition ${
                      copied === l.key
                        ? "bg-brand-600 text-brand-ink"
                        : "bg-neutral-800 text-neutral-300 hover:bg-neutral-700"
                    }`}
                  >
                    <i className={`fa-solid ${copied === l.key ? "fa-check" : "fa-copy"}`} />
                  </button>
                </div>
              </div>
            ))}
          </div>

          <div className="mb-4">
            <div className="text-[10px] text-neutral-600 mb-1.5">
              {t.imageDetail.otherSizes}
              <span className="ml-1 text-faint">{t.imageDetail.otherSizesHint}</span>
            </div>
            <div className="flex flex-wrap gap-1.5">
              {SIZES.map((w) => {
                const key = `w${w}`;
                const url = variants[key];
                if (url) {
                  return (
                    <a
                      key={key}
                      href={url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 rounded-md bg-brand-600/20 border border-brand-500/30 px-2 py-1 text-[10px] text-brand-300 hover:bg-brand-600/30 transition"
                    >
                      <i className="fa-solid fa-check text-[8px]" />
                      {w}px
                    </a>
                  );
                }
                return (
                  <button
                    key={key}
                    onClick={() => makeSize(w)}
                    disabled={making !== null}
                    className="inline-flex items-center justify-center gap-1 rounded-md bg-neutral-800 px-2 py-1 text-[10px] text-neutral-300 hover:bg-neutral-700 disabled:opacity-50 transition min-w-[3.5rem]"
                  >
                    {making === w ? <Spinner className="h-3 w-3 text-brand-400" /> : <>{t.imageDetail.generateSize(w)}</>}
                  </button>
                );
              })}
              {/* The kept original, when the site is configured to store it.
                  Not in SIZES because it is never generated on demand — it
                  either was saved at upload or it is gone. */}
              {originalKey && variants[originalKey] && (
                <a
                  href={variants[originalKey]}
                  target="_blank"
                  rel="noopener noreferrer"
                  title={t.imageDetail.originalTitle}
                  className="inline-flex items-center gap-1 rounded-md bg-amber-600/15 border border-amber-500/30 px-2 py-1 text-[10px] text-amber-300 hover:bg-amber-600/25 transition"
                >
                  <i className="fa-solid fa-file-arrow-down text-[8px]" />
                  {t.imageDetail.originalLink(originalKey.slice(5).toUpperCase())}
                </a>
              )}
              {img.variants.includes("webp") && variants.webp && (
                <a
                  href={variants.webp}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="rounded-md bg-neutral-800 px-2 py-1 text-[10px] text-neutral-300 hover:bg-neutral-700 transition"
                >
                  WebP
                </a>
              )}
              {img.variants.includes("avif") && variants.avif && (
                <a
                  href={variants.avif}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="rounded-md bg-neutral-800 px-2 py-1 text-[10px] text-neutral-300 hover:bg-neutral-700 transition"
                >
                  AVIF
                </a>
              )}
            </div>
            {genErr && <div className="mt-1.5 text-[10px] text-red-400">{genErr}</div>}
          </div>

          <div className="flex items-center gap-2">
            <a
              href={img.url}
              download={img.orig_name}
              className="flex-1 text-center rounded-xl bg-neutral-800 px-4 py-2 text-xs text-neutral-200 hover:bg-neutral-700 transition"
            >
              <i className="fa-solid fa-download mr-1.5" />
              {t.common.download}
            </a>
            <button
              onClick={remove}
              disabled={busy}
              className="flex-1 rounded-xl bg-red-600 px-4 py-2 text-xs font-medium text-white hover:bg-red-700 disabled:opacity-50 transition"
            >
              <i className="fa-solid fa-trash-can mr-1.5" />
              {t.common.delete}
            </button>
          </div>

          <div className="mt-2 text-center">
            <button
              onClick={() => setReporting(true)}
              className="text-[11px] text-neutral-600 hover:text-red-400 transition"
            >
              <i className="fa-solid fa-flag mr-1 text-[9px]" />
              {t.report.imageTitle}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <dt className="text-neutral-600 shrink-0">{label}</dt>
      <dd className="text-neutral-300 truncate">{value}</dd>
    </div>
  );
}

/**
 * Abuse report.
 *
 * Open to anyone, signed in or not — an image host that only accepts reports
 * from account holders finds out about its worst content from its upstream
 * provider instead. Contact is optional for the same reason: requiring it
 * filters out exactly the people least willing to attach their name.
 */
function ReportDialog({ img, onClose }: { img: Image; onClose: () => void }) {
  const { t } = useLang();
  const toast = useToast();
  const [reason, setReason] = useState("");
  const [preset, setPreset] = useState<(typeof CATEGORIES)[number] | null>(null);
  const [contact, setContact] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  // Category keys, not labels. The old code compared `preset !== "其他"`
  // against the displayed text, which stops matching the moment the text is
  // translated — the picker would keep behaving as if a category were chosen.
  const CATEGORIES = ["porn", "copyright", "violence", "privacy", "fraud", "other"] as const;
  const label = (k: (typeof CATEGORIES)[number]) => t.report.category[k];
  const full = preset && preset !== "other" ? `${label(preset)}${reason ? " — " + reason : ""}` : reason;
  const ready = (full || "").trim().length >= 4;

  async function submit() {
    if (!ready || busy) return;
    setBusy(true);
    setErr(null);
    try {
      await reportApi.submit(img.id, full!.trim(), contact.trim());
      onClose();
      toast.success(t.report.submitted, t.report.submittedDetail);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center px-4">
      <div className="absolute inset-0 bg-black/50" onClick={() => !busy && onClose()} />
      <div className="relative w-full max-w-sm rounded-2xl border border-neutral-800 bg-neutral-900 p-5 shadow-panel">
        <div className="flex items-center gap-2 mb-1">
          <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-red-900/40 text-xs text-red-300">
            <i className="fa-solid fa-flag" />
          </span>
          <span className="text-sm text-neutral-100">{t.report.dialogTitle}</span>
        </div>
        <p className="text-[11px] leading-relaxed text-neutral-500 mb-3">
          {t.report.notice}
        </p>

        <div className="flex flex-wrap gap-1.5 mb-3">
          {CATEGORIES.map((k) => (
            <button
              key={k}
              onClick={() => setPreset(preset === k ? null : k)}
              className={`inline-flex h-8 items-center justify-center rounded-lg px-3 text-xs transition ${
                preset === k
                  ? "bg-red-600/20 text-red-300 border border-red-500/40"
                  : "bg-neutral-800 text-neutral-400 hover:text-neutral-100 border border-transparent"
              }`}
            >
              {label(k)}
            </button>
          ))}
        </div>

        <textarea
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          rows={3}
          maxLength={500}
          placeholder={preset && preset !== "other" ? t.report.detailsOptionalPlaceholder : t.report.reasonPlaceholder}
          className="w-full rounded-lg bg-neutral-950 border border-neutral-800 px-2.5 py-2 text-xs outline-none focus:border-red-500 placeholder-faint resize-none"
        />

        <input
          value={contact}
          onChange={(e) => setContact(e.target.value)}
          placeholder={t.report.contactPlaceholder}
          className="mt-2 w-full h-8 rounded-lg bg-neutral-950 border border-neutral-800 px-2.5 text-xs outline-none focus:border-red-500 placeholder-faint"
        />

        {err && <div className="mt-2 text-[11px] text-red-400">{err}</div>}

        <div className="mt-3 flex items-center gap-2">
          <div className="flex-1" />
          <button
            onClick={onClose}
            disabled={busy}
            className="inline-flex h-8 items-center justify-center rounded-lg px-3 text-xs text-neutral-400 hover:text-neutral-100 transition"
          >
            {t.common.cancel}
          </button>
          <button
            onClick={submit}
            disabled={!ready || busy}
            className="inline-flex h-8 items-center justify-center rounded-lg bg-red-600 px-3 text-xs font-medium text-white hover:bg-red-700 disabled:bg-neutral-800 disabled:text-neutral-500 transition"
          >
            {busy ? <RingSpinner className="h-3.5 w-3.5" /> : t.report.submit}
          </button>
        </div>
      </div>
    </div>
  );
}
