import Icon from "../../Icon";
import { useState } from "react";
import { Link } from "react-router-dom";
import { useLang } from "../../LangContext";
import { useToast } from "../../ToastContext";
import { useDialog } from "../../DialogContext";
import { RingSpinner } from "../Spinner";
import { genKind, genSources, inFlight, picbiRemaining } from "./generations";
import type { AIGeneration, AIStatusOn, Image } from "../../types";

/**
 * The pieces both AI pages draw with: the allowance readout, the option chips
 * and the polled history list.
 *
 * They were lifted out of pages/Generate.tsx unchanged in behaviour, comments
 * included, and the retouch page reuses them rather than owning a second copy
 * that would drift. The non-visual half — the polling hook, the caps, the
 * helpers for reading a record — is next door in generations.ts.
 */

/**
 * The allowance, compact: one line of text for the composer's bottom row.
 *
 * It used to be a card of its own beside the composer — a heading, a 3xl
 * number, two rows of labels and a progress bar, all for three figures. The
 * bottom row (between the resolution chips and the submit button) was empty at
 * the same time, and "how many do I have left" is the last thing anyone checks
 * before pressing generate, so the figures moved to where that question gets
 * asked. The bar went with the card: `0 / 5` already says what the bar was
 * approximating, and more precisely.
 *
 * Nothing is drawn when there is nothing left to spend — that case is not a
 * number, it is a thing to do about it, and AIQuotaNotice says which.
 */
export function QuotaInline({ status }: { status: AIStatusOn }) {
  const { t } = useLang();
  const q = t.generate.quota.inline;

  const picbi = picbiRemaining(status);
  const local = status.remaining;
  if (local <= 0 && picbi <= 0) return null;

  // 额度有了有效期,就得说清哪天作废——不说的话用户是在毫无预警的情况下
  // 少掉几次。放 title 里而不是单开一段:它只在快到期时才有意义,常态下多
  // 一段字反而把这一行挤散。
  const expiry =
    status.next_expiry && status.next_expiry_credits
      ? q.expiring(status.next_expiry_credits, new Date(status.next_expiry).toLocaleDateString())
      : undefined;

  // The lead figure is whatever the next generation will actually be drawn
  // from. Once the free allowance is spent the pic.bi balance is the pool in
  // use, and showing a brand-coloured "0 次" above a button that still works
  // would be describing a wall that is not there.
  const lead = local > 0 ? q.times(local) : q.picbi(picbi);

  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] tabular-nums">
      <span className="font-medium text-brand-300">{lead}</span>
      <span className="text-neutral-700">·</span>
      <span className="text-neutral-500">{q.today(status.used_today, status.daily_limit)}</span>
      <span className="text-neutral-700">·</span>
      <span className="text-neutral-500">{q.monthly(status.monthly_left, status.monthly)}</span>
      {status.granted > 0 && (
        <>
          <span className="text-neutral-700">·</span>
          <span className="text-neutral-500" title={expiry}>
            {q.granted(status.granted)}
          </span>
        </>
      )}
      {/* Only worth a fourth segment while the free pool is still the one in
          use; once it is not, the pic.bi figure is already the lead. */}
      {local > 0 && picbi > 0 && (
        <>
          <span className="text-neutral-700">·</span>
          <span className="text-neutral-500">{q.picbi(picbi)}</span>
        </>
      )}
    </div>
  );
}

/**
 * The one line that appears only when nothing can be spent.
 *
 * Which wall you hit decides what to do about it, so the two are never
 * collapsed into one "out of quota" — but only one of them is ever shown, and
 * the monthly one wins. A month at zero is a month at zero tomorrow as well,
 * so "come back tomorrow" there is a sentence that costs the reader a day;
 * "check in for more" is the only one of the two that is still actionable.
 *
 * A linked pic.bi with credits on it means neither wall is up — the server
 * falls through to that ledger — so this renders nothing at all.
 */
export function QuotaNotice({ status }: { status: AIStatusOn }) {
  const { t } = useLang();
  const q = t.generate.quota;

  if (status.remaining > 0 || picbiRemaining(status) > 0) return null;
  const monthly = status.credits <= 0;

  return (
    <div className="mb-3 flex flex-wrap items-center gap-x-2 gap-y-1 rounded-xl border border-amber-500/30 bg-amber-950/20 px-3.5 py-2.5 text-[11px] text-amber-200">
      <Icon name={monthly ? "hourglass-end" : "clock"} className={`mr-0.5`}  />
      <span>{monthly ? q.monthlyExhausted : q.dailyExhausted}</span>
      <span className="text-amber-200/70">
        {monthly ? q.monthlyExhaustedHint : q.dailyExhaustedHint(status.daily_limit)}
      </span>
      {monthly && (
        <Link to="/space" className="text-brand-400 hover:underline">
          {q.checkinLink}
        </Link>
      )}
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
            <Icon name="arrows-left-right-to-line" className="text-[9px]"  />
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
  onDelete,
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
  onDelete: (g: AIGeneration, alsoImage: boolean) => void;
  resolveSource?: (id: string) => Image | undefined;
}) {
  const { t, locale } = useLang();
  const toast = useToast();
  const dialog = useDialog();
  const [copied, setCopied] = useState<string | null>(null);

  // 有产出图时是三选一,没有就是普通确认——与其给一个只有一种结果可选的
  // 勾选框,不如让按钮本身说清会发生什么。
  async function askDelete(g: AIGeneration, img?: Image) {
    const h = t.generate.history;
    if (!img) {
      if (await dialog.confirm({ title: h.removeTitle, body: h.removeBody, danger: true,
                                 confirmLabel: t.common.delete })) {
        onDelete(g, false);
      }
      return;
    }
    const pick = await dialog.choose({
      title: h.removeTitle,
      body: h.removeBody,
      options: [
        { label: h.removeKeepImage, value: "keep" },
        { label: h.removeWithImage, value: "image", danger: true },
      ],
    });
    if (pick === "keep") onDelete(g, false);
    if (pick === "image") onDelete(g, true);
  }

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
            <Icon name={icon} className={`text-xl text-brand-500`}  />
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
                        <Icon name="wand-magic" className="mr-1 text-[9px]"  />
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
                      <Icon name="rotate-left" className="mr-1 text-[9px]"  />
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
                          <Icon
                            name={copied === img.id ? "check" : "link"}
                            className="mr-1 text-[9px]"
                          />
                          {copied === img.id ? t.common.copied : t.generate.history.copyLink}
                        </button>
                        <button
                          onClick={() => onOpenDetail(img)}
                          className="text-[10px] text-neutral-600 hover:text-brand-300 transition"
                        >
                          <Icon name="up-right-and-down-left-from-center" className="mr-1 text-[9px]"  />
                          {t.generate.history.openDetail}
                        </button>
                      </>
                    )}

                    {/* 还在跑的不给删:额度已经扣了、上游可能还在出图,让它从
                        界面上消失就等于让一笔未结的账消失。 */}
                    {!inFlight(g.status) && (
                      <button
                        onClick={() => askDelete(g, img)}
                        className="text-[10px] text-neutral-600 hover:text-red-300 transition"
                      >
                        <Icon name="trash-can" className="mr-1 text-[9px]"  />
                        {t.generate.history.remove}
                      </button>
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
        <Icon name="triangle-exclamation" className="text-sm text-red-500/70"  />
      ) : gen.status === "completed" ? (
        // Completed, but the picture is gone — deleted from the gallery since.
        <Icon name="image" className="text-sm text-neutral-700"  />
      ) : (
        <RingSpinner className="h-4 w-4 text-brand-500" />
      )}
    </div>
  );
}

export function Center({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen flex items-center justify-center text-neutral-500">{children}</div>
  );
}
