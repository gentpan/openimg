import { useState } from "react";
import { Link } from "react-router-dom";
import { formatBytes } from "../../api";
import { useLang } from "../../LangContext";
import { useToast } from "../../ToastContext";
import { RingSpinner } from "../Spinner";
import { genKind, genSources, inFlight } from "./generations";
import type { AIGeneration, AIStatusOn, Image } from "../../types";

/**
 * The pieces both AI pages draw with: the allowance card, the option chips and
 * the polled history list.
 *
 * They were lifted out of pages/Generate.tsx unchanged in behaviour, comments
 * included, and the retouch page reuses them rather than owning a second copy
 * that would drift. The non-visual half — the polling hook, the caps, the
 * helpers for reading a record — is next door in generations.ts.
 */

/**
 * The allowance card. Identical on both pages because it is the same
 * allowance — retouching and generating draw on one pool of credits.
 */
export function QuotaCard({
  status,
  availableBytes,
}: {
  status: AIStatusOn;
  availableBytes: number;
}) {
  const { t } = useLang();

  // Which wall you hit decides what to do about it, so the two are never
  // collapsed into one "out of quota". They are also not exclusive: spending
  // the last of the month on the same day as the fifth of the day hits both,
  // and showing only one of them would send someone off to check in for
  // credits they still could not spend until tomorrow. So each is asked
  // separately, and both notices appear when both are true.
  const blocked = status.remaining <= 0;
  const monthlySpent = status.credits <= 0;
  const dailySpent = status.used_today >= status.daily_limit;

  return (
    <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-5">
      <div className="text-[10px] uppercase tracking-wider text-neutral-600 mb-3">
        {t.generate.quota.title}
      </div>
      <div className="text-3xl font-brand text-neutral-100 tabular-nums">
        {status.remaining}
        <span className="text-xs text-neutral-500 ml-1.5">{t.generate.quota.unit(status.remaining)}</span>
      </div>
      <div className="text-xs text-neutral-600 mt-1 mb-4">{t.generate.quota.remaining}</div>

      <dl className="space-y-1 text-[11px]">
        <Row
          label={t.generate.quota.today}
          value={t.generate.quota.todayValue(status.used_today, status.daily_limit)}
        />
        <Row
          label={t.generate.quota.monthly}
          value={t.generate.quota.monthlyValue(status.credits, status.monthly)}
        />
      </dl>

      <div className="mt-3 h-1 rounded-full bg-neutral-800 overflow-hidden">
        <div
          className="h-full bg-brand-600 transition-all"
          style={{
            width: `${status.monthly > 0 ? Math.min(100, (status.credits / status.monthly) * 100) : 0}%`,
          }}
        />
      </div>
      <div className="mt-1.5 text-[10px] text-faint">{t.generate.quota.resetNote}</div>

      {/* Monthly first when both are up: its remedy is the one that takes
          a day to act on, so it is the one to start on. */}
      {blocked && monthlySpent && (
        <div className="mt-4 rounded-xl border border-amber-500/30 bg-amber-950/20 px-3 py-2.5 text-[11px] text-amber-200">
          <div>
            <i className="fa-solid fa-hourglass-end mr-1.5" />
            {t.generate.quota.monthlyExhausted}
          </div>
          <div className="mt-1 text-amber-200/70">{t.generate.quota.monthlyExhaustedHint}</div>
          <Link to="/space" className="mt-1.5 inline-block text-brand-400 hover:underline">
            {t.generate.quota.checkinLink}
          </Link>
        </div>
      )}
      {blocked && dailySpent && (
        <div className="mt-4 rounded-xl border border-amber-500/30 bg-amber-950/20 px-3 py-2.5 text-[11px] text-amber-200">
          <div>
            <i className="fa-solid fa-clock mr-1.5" />
            {t.generate.quota.dailyExhausted}
          </div>
          <div className="mt-1 text-amber-200/70">
            {t.generate.quota.dailyExhaustedHint(status.daily_limit)}
          </div>
        </div>
      )}
      {/* Storage is the other thing a generated picture spends, and it is
          not on this card anywhere else. */}
      <div className="mt-4 flex items-center justify-between text-[11px]">
        <span className="text-neutral-600">{t.common.availableStorage}</span>
        <Link to="/space" className="text-neutral-300 hover:text-brand-300 transition">
          {formatBytes(availableBytes)}
        </Link>
      </div>
    </div>
  );
}

/**
 * One row of mutually exclusive chips.
 *
 * `autoLabel` adds a leading "leave it to the server" chip carrying `null`.
 * Text-to-image has no such option — an image has to come out at *some* ratio,
 * and the server picks one anyway — but retouching does: omit the size and the
 * output keeps the shape of the picture that went in, which is what someone
 * removing a watermark almost always wants.
 */
export function OptionPicker({
  label,
  options,
  value,
  onChange,
  autoLabel,
  autoTitle,
  ratio = false,
  uppercase = false,
}: {
  label: string;
  options: string[];
  value: string | null;
  onChange: (v: string | null) => void;
  autoLabel?: string;
  autoTitle?: string;
  ratio?: boolean;
  uppercase?: boolean;
}) {
  const chip = (active: boolean) =>
    `inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-[11px] transition ${
      active
        ? "border-brand-500 bg-brand-950/40 text-brand-300"
        : "border-neutral-800 bg-neutral-900 text-neutral-400 hover:border-neutral-700 hover:text-neutral-200"
    }`;

  return (
    <div>
      <div className="mb-1.5 text-[10px] uppercase tracking-wider text-neutral-600">{label}</div>
      <div className="flex flex-wrap gap-1.5">
        {autoLabel && (
          <button onClick={() => onChange(null)} title={autoTitle} className={chip(value === null)}>
            <i className="fa-solid fa-arrows-left-right-to-line text-[9px]" />
            {autoLabel}
          </button>
        )}
        {options.map((o) => (
          <button
            key={o}
            onClick={() => onChange(o)}
            className={`${chip(o === value)} ${uppercase ? "uppercase" : ""}`}
          >
            {ratio && <RatioIcon ratio={o} active={o === value} />}
            {o}
          </button>
        ))}
      </div>
    </div>
  );
}

/**
 * The ratio drawn rather than only named. "3:2" and "2:3" are one character
 * apart and pick opposite orientations, which is a mistake worth making
 * impossible at a glance.
 */
export function RatioIcon({ ratio, active }: { ratio: string; active: boolean }) {
  const [a, b] = ratio.split(":").map((n) => Number(n) || 1);
  const max = Math.max(a, b);
  return (
    <span className="flex h-4 w-4 items-center justify-center">
      <span
        className={`block rounded-[2px] border ${active ? "border-brand-400" : "border-neutral-600"}`}
        style={{ width: `${(a / max) * 100}%`, height: `${(b / max) * 100}%` }}
      />
    </span>
  );
}

/**
 * The polled history list.
 *
 * Serves both pages: an edit row is a generation row that also names what it
 * started from, and `resolveSource` is how it gets there — the generations
 * endpoint only promises the *result* image in its map, so a caller that knows
 * more (the retouch page remembers what the user picked) can supply it, and a
 * caller that does not falls back to saying how many sources there were.
 */
export function GenerationHistory({
  gens,
  images,
  working,
  title,
  empty,
  emptyHint,
  icon,
  reuseLabel,
  onReuse,
  onOpenDetail,
  resolveSource,
}: {
  gens: AIGeneration[];
  images: Record<string, Image>;
  working: boolean;
  title: string;
  empty: string;
  emptyHint: string;
  icon: string;
  reuseLabel: string;
  onReuse: (g: AIGeneration) => void;
  onOpenDetail: (img: Image) => void;
  resolveSource?: (id: string) => Image | undefined;
}) {
  const { t, locale } = useLang();
  const toast = useToast();
  const [copied, setCopied] = useState<string | null>(null);

  async function copyLink(img: Image) {
    try {
      await navigator.clipboard.writeText(img.short_url || img.url);
      setCopied(img.id);
      setTimeout(() => setCopied((c) => (c === img.id ? null : c)), 1500);
      toast.success(t.common.copied);
    } catch {
      // Clipboard denied — the detail panel has a selectable field.
    }
  }

  return (
    <div className="mt-8 rounded-2xl border border-neutral-800 bg-neutral-900/40 overflow-hidden">
      <div className="flex items-center gap-2 px-5 py-3 border-b border-neutral-800/60">
        <span className="text-xs text-neutral-300">{title}</span>
        {gens.length > 0 && (
          <span className="text-[10px] text-neutral-600">{t.generate.history.count(gens.length)}</span>
        )}
        {working && <RingSpinner className="h-3 w-3 text-brand-400" />}
      </div>

      {gens.length === 0 ? (
        <div className="px-5 py-14 text-center">
          <div className="inline-flex items-center justify-center w-14 h-14 rounded-2xl bg-neutral-900 mb-3">
            <i className={`fa-solid ${icon} text-xl text-brand-500`} />
          </div>
          <div className="text-sm text-neutral-400">{empty}</div>
          <div className="mt-1 text-xs text-neutral-600">{emptyHint}</div>
        </div>
      ) : (
        <div className="divide-y divide-neutral-800/50">
          {gens.map((g) => {
            const img = g.image_id ? images[g.image_id] : undefined;
            const sources = genSources(g);
            const meta = [g.size, g.resolution ? g.resolution.toUpperCase() : ""].filter(Boolean);
            return (
              <div key={g.id} className="flex items-start gap-3 px-5 py-3">
                <Thumb
                  gen={g}
                  img={img}
                  onOpen={() => img && onOpenDetail(img)}
                  openTitle={t.generate.history.openDetail}
                />

                <div className="flex-1 min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <StatusBadge status={g.status} label={t.generate.history.status[g.status]} />
                    {genKind(g) === "edit" && (
                      <span className="rounded-full bg-neutral-800 px-2 py-0.5 text-[10px] text-neutral-300">
                        <i className="fa-solid fa-wand-magic mr-1 text-[9px]" />
                        {t.retouch.history.kindBadge}
                      </span>
                    )}
                    {meta.length > 0 && (
                      <span className="text-[10px] text-neutral-600">{meta.join(" · ")}</span>
                    )}
                    <span className="text-[10px] text-faint">
                      {new Date(g.created_at).toLocaleString(locale)}
                    </span>
                  </div>

                  {sources.length > 0 && (
                    <SourceStrip
                      ids={sources}
                      resolve={resolveSource}
                      onOpen={onOpenDetail}
                    />
                  )}

                  <div className="mt-1 text-xs text-neutral-300 line-clamp-2 break-words" title={g.prompt}>
                    {g.prompt}
                  </div>

                  {inFlight(g.status) && (
                    <div className="mt-1 text-[10px] text-neutral-600">{t.generate.history.working}</div>
                  )}

                  {g.status === "failed" && (
                    <div className="mt-1.5 rounded-lg border border-red-500/25 bg-red-950/20 px-2.5 py-1.5 text-[10px] text-red-300">
                      <span className="text-red-400/70">{t.generate.history.failedLabel}: </span>
                      {g.error || t.generate.history.status.failed}
                      <span className="ml-1.5 text-neutral-500">· {t.generate.history.refunded}</span>
                    </div>
                  )}

                  <div className="mt-1.5 flex flex-wrap items-center gap-2">
                    <button
                      onClick={() => onReuse(g)}
                      title={reuseLabel}
                      className="text-[10px] text-neutral-600 hover:text-brand-300 transition"
                    >
                      <i className="fa-solid fa-rotate-left mr-1 text-[9px]" />
                      {reuseLabel}
                    </button>

                    {img && (
                      <>
                        <button
                          onClick={() => copyLink(img)}
                          className={`text-[10px] transition ${
                            copied === img.id ? "text-brand-300" : "text-neutral-600 hover:text-brand-300"
                          }`}
                        >
                          <i
                            className={`fa-solid mr-1 text-[9px] ${
                              copied === img.id ? "fa-check" : "fa-link"
                            }`}
                          />
                          {copied === img.id ? t.common.copied : t.generate.history.copyLink}
                        </button>
                        <button
                          onClick={() => onOpenDetail(img)}
                          className="text-[10px] text-neutral-600 hover:text-brand-300 transition"
                        >
                          <i className="fa-solid fa-up-right-and-down-left-from-center mr-1 text-[9px]" />
                          {t.generate.history.openDetail}
                        </button>
                      </>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

/**
 * What an edit started from.
 *
 * Sources are ids, and an id only becomes a thumbnail if the caller can still
 * resolve it. Two things break that: the record outlives the picture, and this
 * session may simply never have loaded it. Either way the row still says how
 * many pictures went in rather than quietly rendering nothing, because "one
 * source or three" is most of what the reader wanted from this line.
 */
function SourceStrip({
  ids,
  resolve,
  onOpen,
}: {
  ids: string[];
  resolve?: (id: string) => Image | undefined;
  onOpen: (img: Image) => void;
}) {
  const { t } = useLang();
  const found = ids.map((id) => resolve?.(id)).filter((i): i is Image => !!i);

  return (
    <div className="mt-1.5 flex items-center gap-1.5">
      <span className="text-[10px] text-neutral-600 shrink-0">{t.retouch.history.sourceLabel}</span>
      {found.length > 0 ? (
        found.map((img) => (
          <button
            key={img.id}
            onClick={() => onOpen(img)}
            title={img.orig_name}
            className="w-7 h-7 rounded-md overflow-hidden border border-neutral-800 hover:border-brand-500 transition"
          >
            <img src={img.thumb_url} alt={img.orig_name} loading="lazy" className="w-full h-full object-cover" />
          </button>
        ))
      ) : (
        <span className="text-[10px] text-neutral-600">{t.retouch.history.sourceUnavailable(ids.length)}</span>
      )}
      {found.length > 0 && found.length < ids.length && (
        <span className="text-[10px] text-neutral-600">
          {t.retouch.history.sourceMissing(ids.length - found.length)}
        </span>
      )}
    </div>
  );
}

export function StatusBadge({ status, label }: { status: AIGeneration["status"]; label: string }) {
  const tone =
    status === "completed"
      ? "bg-brand-950/40 text-brand-300"
      : status === "failed"
        ? "bg-red-950/40 text-red-300"
        : "bg-neutral-800 text-neutral-400";
  return (
    <span className={`rounded-full px-2 py-0.5 text-[10px] ${tone}`}>
      {inFlight(status) && <RingSpinner className="h-2.5 w-2.5 inline-block align-[-1px] mr-1" />}
      {label}
    </span>
  );
}

/** Square preview slot: the picture once it exists, its state before that. */
function Thumb({
  gen,
  img,
  onOpen,
  openTitle,
}: {
  gen: AIGeneration;
  img?: Image;
  onOpen: () => void;
  openTitle: string;
}) {
  if (img) {
    return (
      <button
        onClick={onOpen}
        title={openTitle}
        className="shrink-0 w-16 h-16 rounded-xl overflow-hidden border border-neutral-800 hover:border-brand-500 transition"
      >
        <img src={img.thumb_url} alt={gen.prompt} loading="lazy" className="w-full h-full object-cover" />
      </button>
    );
  }
  return (
    <div className="shrink-0 w-16 h-16 rounded-xl border border-neutral-800 bg-neutral-950 flex items-center justify-center">
      {gen.status === "failed" ? (
        <i className="fa-solid fa-triangle-exclamation text-sm text-red-500/70" />
      ) : gen.status === "completed" ? (
        // Completed, but the picture is gone — deleted from the gallery since.
        <i className="fa-solid fa-image text-sm text-neutral-700" />
      ) : (
        <RingSpinner className="h-4 w-4 text-brand-500" />
      )}
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between">
      <dt className="text-neutral-600">{label}</dt>
      <dd className="text-neutral-300">{value}</dd>
    </div>
  );
}

export function Center({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen flex items-center justify-center text-neutral-500">{children}</div>
  );
}
