import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { imageApi } from "../../api";
import { useLang } from "../../LangContext";
import { RingSpinner } from "../Spinner";
import type { Image } from "../../types";

/** One screenful of the picker's grid: a multiple of the 4-column layout, so a
 *  full page never ends in a short row with holes where cards would be. */
const PAGE = 24;

/**
 * Picks pictures out of the user's own gallery — sources for a retouch, or
 * references for a generation.
 *
 * Not a file input, and deliberately still not one: what upstream needs is a
 * public URL, so a picture that is already hosted here needs nothing done to
 * it. Local files reach the same place from the row that opens this dialog
 * (components/ai/SourceImages) — they go up through the ordinary upload
 * pipeline first and arrive here as ordinary gallery images.
 *
 * Selection is an ordered list rather than a set: the order the ids go up is
 * the order the model reads them in, and for a two-image edit ("put the subject
 * from the first into the second") that order is the instruction.
 */
export default function ImagePicker({
  initial,
  max,
  onCancel,
  onConfirm,
}: {
  initial: Image[];
  max: number;
  onCancel: () => void;
  onConfirm: (picked: Image[]) => void;
}) {
  const { t } = useLang();
  const [picked, setPicked] = useState<Image[]>(initial);
  const [images, setImages] = useState<Image[]>([]);
  const [total, setTotal] = useState(0);
  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const load = useCallback(async (offset: number, q: string, append: boolean) => {
    setBusy(true);
    try {
      const res = await imageApi.list({ limit: PAGE, offset, q: q || undefined, sort: "newest" });
      setImages((prev) => (append ? [...prev, ...res.images] : res.images));
      setTotal(res.total);
      setErr(null);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }, []);

  // Debounced search, which also covers the first load.
  useEffect(() => {
    const timer = setTimeout(() => load(0, query, false), query ? 300 : 0);
    return () => clearTimeout(timer);
  }, [query, load]);

  // Escape closes. A modal that traps the reader with no keyboard way out is
  // the one thing worse than no modal.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onCancel();
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onCancel]);

  function toggle(img: Image) {
    setPicked((prev) => {
      const at = prev.findIndex((p) => p.id === img.id);
      if (at >= 0) return prev.filter((p) => p.id !== img.id);
      // Silently ignored rather than disabled-with-no-explanation: the cap is
      // spelled out in the header and the counter turns amber on the way up.
      if (prev.length >= max) return prev;
      return [...prev, img];
    });
  }

  const full = picked.length >= max;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center px-4 py-6">
      <div className="absolute inset-0 bg-black/60" onClick={onCancel} />

      <div className="relative flex w-full max-w-4xl max-h-full flex-col rounded-2xl border border-neutral-800 bg-neutral-900 shadow-panel">
        <div className="flex flex-wrap items-center gap-3 border-b border-neutral-800 px-5 py-3">
          <div className="min-w-0">
            <div className="text-sm text-neutral-100">{t.aiSource.picker.title}</div>
            <div className="text-[11px] text-neutral-600">{t.aiSource.picker.subtitle(max)}</div>
          </div>

          <div className="flex-1" />

          <div className="relative">
            <i className="fa-solid fa-magnifying-glass absolute left-3 top-1/2 -translate-y-1/2 text-[10px] text-neutral-600" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t.gallery.searchPlaceholder}
              className="w-44 rounded-lg border border-neutral-800 bg-neutral-950 py-1.5 pl-8 pr-3 text-xs outline-none transition focus:border-brand-500"
            />
          </div>

          <button
            onClick={onCancel}
            title={t.common.close}
            aria-label={t.common.close}
            className="text-neutral-500 transition hover:text-neutral-100"
          >
            <i className="fa-solid fa-xmark" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4">
          {err && <div className="mb-3 text-[11px] text-red-400">{err}</div>}

          {images.length === 0 && !busy ? (
            <div className="py-16 text-center">
              <div className="mb-3 inline-flex h-14 w-14 items-center justify-center rounded-2xl bg-neutral-950">
                <i className="fa-solid fa-images text-xl text-brand-500" />
              </div>
              <div className="text-sm text-neutral-400">
                {query ? t.gallery.emptySearch : t.common.noUploadsYet}
              </div>
              {!query && (
                <Link to="/upload" className="mt-3 inline-block text-xs text-brand-400 hover:underline">
                  {t.common.uploadFirst}
                </Link>
              )}
            </div>
          ) : (
            <>
              <div className="grid grid-cols-3 gap-3 sm:grid-cols-4 lg:grid-cols-6">
                {images.map((img) => {
                  const at = picked.findIndex((p) => p.id === img.id);
                  return (
                    <PickCard
                      key={img.id}
                      img={img}
                      order={at >= 0 ? at + 1 : null}
                      // Everything stays clickable while full, because one of
                      // those cards is a picked one and clicking it is how you
                      // make room.
                      dimmed={full && at < 0}
                      onToggle={() => toggle(img)}
                    />
                  );
                })}
              </div>

              {images.length < total && (
                <div className="mt-4 text-center">
                  <button
                    onClick={() => load(images.length, query, true)}
                    disabled={busy}
                    className="rounded-xl border border-neutral-800 bg-neutral-950 px-5 py-2 text-xs text-neutral-300 transition hover:border-neutral-700 disabled:opacity-50"
                  >
                    {busy ? (
                      <RingSpinner className="inline-block h-3.5 w-3.5 align-[-2px]" />
                    ) : (
                      t.gallery.loadMore
                    )}
                  </button>
                </div>
              )}
            </>
          )}
        </div>

        <div className="flex items-center gap-3 border-t border-neutral-800 px-5 py-3">
          <span className={`text-[11px] tabular-nums ${full ? "text-amber-300" : "text-neutral-500"}`}>
            {t.aiSource.picker.selected(picked.length, max)}
          </span>
          {full && <span className="text-[10px] text-neutral-600">{t.aiSource.picker.limitReached(max)}</span>}

          <div className="flex-1" />

          <button
            onClick={onCancel}
            className="rounded-xl border border-neutral-800 px-4 py-2 text-xs text-neutral-400 transition hover:border-neutral-700 hover:text-neutral-200"
          >
            {t.common.cancel}
          </button>
          <button
            onClick={() => onConfirm(picked)}
            disabled={picked.length === 0}
            className="rounded-xl bg-brand-600 px-4 py-2 text-xs font-medium text-brand-ink transition hover:bg-brand-500 disabled:bg-neutral-800 disabled:text-neutral-600"
          >
            {t.aiSource.picker.confirm}
          </button>
        </div>
      </div>
    </div>
  );
}

/**
 * One candidate. The whole card is the toggle — the gallery keeps a separate
 * checkbox because clicking a card there opens the picture, but here there is
 * nothing else a click could mean.
 */
function PickCard({
  img,
  order,
  dimmed,
  onToggle,
}: {
  img: Image;
  order: number | null;
  dimmed: boolean;
  onToggle: () => void;
}) {
  const { t } = useLang();
  const selected = order !== null;
  return (
    <button
      onClick={onToggle}
      title={selected ? t.gallery.deselect : img.orig_name}
      className={`group relative aspect-square overflow-hidden rounded-xl border transition ${
        selected ? "border-brand-500" : "border-neutral-800 hover:border-neutral-700"
      } ${dimmed ? "opacity-40" : ""}`}
    >
      <img src={img.thumb_url} alt={img.orig_name} loading="lazy" className="h-full w-full object-cover" />

      {/* The badge carries the position, not a tick: with up to four sources
          the order is part of the instruction, so it has to be readable off
          the grid rather than inferred from the order you happened to click. */}
      <span
        className={`absolute left-2 top-2 flex h-5 w-5 items-center justify-center rounded-full text-[10px] tabular-nums transition ${
          selected
            ? "bg-brand-600 text-brand-ink"
            : "border border-white/50 bg-black/50 text-transparent group-hover:text-white"
        }`}
      >
        {selected ? order : <i className="fa-solid fa-check text-[9px]" />}
      </span>
    </button>
  );
}
