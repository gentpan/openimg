import { useCallback, useEffect, useRef, useState } from "react";
import { Link, Navigate } from "react-router-dom";
import { useAuth } from "../AuthContext";
import Footer from "../Footer";
import Nav from "../components/Nav";
import ImageDetail from "../components/ImageDetail";
import { RingSpinner } from "../components/Spinner";
import { aiApi, formatBytes } from "../api";
import { refreshAIStatus, useAIStatus } from "../aiStatus";
import { useLang } from "../LangContext";
import { useToast } from "../ToastContext";
import type { AIGeneration, Image } from "../types";

/** Mirrors the server's own cap. Enforced here so the counter can turn red
 *  before a submission is rejected for a reason nothing on screen showed. */
const MAX_PROMPT = 1000;

/** Generation takes tens of seconds; three seconds is frequent enough to feel
 *  live without turning one picture into twenty requests. */
const POLL_MS = 3000;

/**
 * Text to image.
 *
 * The page exists only where the deployment configured an upstream key — the
 * status endpoint answers `{enabled: false}` otherwise and both this route and
 * its nav entry disappear, because a disabled feature that is still visible
 * reads as something broken rather than something absent.
 *
 * Everything after submission is polled: the POST returns a `pending` record,
 * and the history below is refetched until that record settles. Polling stops
 * the moment nothing is in flight, so an idle page is silent.
 */
export default function GeneratePage() {
  const { t, locale } = useLang();
  const toast = useToast();
  const { user, loading, refresh } = useAuth();
  const { status, loading: statusLoading } = useAIStatus(!!user);

  const [prompt, setPrompt] = useState("");
  // Null until the user picks. The valid set arrives with the status, so the
  // effective choice is derived below rather than stored and then corrected —
  // storing it would mean rendering buttons for a value the server will
  // silently rewrite to 1:1 / 1k, and the highlighted button would be a lie.
  const [size, setSize] = useState<string | null>(null);
  const [resolution, setResolution] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const [gens, setGens] = useState<AIGeneration[]>([]);
  const [images, setImages] = useState<Record<string, Image>>({});
  const [detail, setDetail] = useState<Image | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  const promptRef = useRef<HTMLTextAreaElement>(null);

  const load = useCallback(async () => {
    const r = await aiApi.generations();
    setGens(r.generations);
    setImages(r.images);
  }, []);

  useEffect(() => {
    if (!user) return;
    load().catch(() => {});
    // The cached status can be stale in either direction — a check-in grants
    // credits, a failed generation refunds one — so arriving here resyncs it.
    refreshAIStatus();
  }, [user, load]);

  const sizes = status?.enabled ? status.sizes : [];
  const resolutions = status?.enabled ? status.resolutions : [];
  const activeSize = size && sizes.includes(size) ? size : (sizes[0] ?? "1:1");
  const activeResolution =
    resolution && resolutions.includes(resolution) ? resolution : (resolutions[0] ?? "1k");

  const active = gens.some((g) => g.status === "pending" || g.status === "running");

  useEffect(() => {
    if (!active) return;
    const h = window.setInterval(() => {
      // A backgrounded tab is nobody watching. The next visible tick catches up.
      if (document.hidden) return;
      load().catch(() => {});
    }, POLL_MS);
    return () => window.clearInterval(h);
  }, [active, load]);

  // The moment the last job settles: a completed picture consumed storage, and
  // a failed one handed a credit back. Both numbers live outside this page.
  const wasActive = useRef(false);
  useEffect(() => {
    if (wasActive.current && !active) {
      refresh();
      refreshAIStatus();
    }
    wasActive.current = active;
  }, [active, refresh]);

  async function submit() {
    const text = prompt.trim();
    if (!text || busy) return;
    setBusy(true);
    setErr(null);
    try {
      const gen = await aiApi.generate(text, activeSize, activeResolution);
      // Prepended rather than refetched: the record is already the server's
      // own, and waiting a round trip to see your own submission appear is
      // exactly the moment a page feels unresponsive.
      setGens((prev) => [gen, ...prev]);
      toast.success(t.generate.submitted, t.generate.submittedDetail);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setErr(msg);
      toast.error(t.generate.submitFailed, msg);
    } finally {
      // Both paths move the counters: a success spends one, and a rejection
      // usually means the numbers on screen were already out of date.
      refreshAIStatus();
      setBusy(false);
    }
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

  if (loading) return <Center>{t.common.loading}</Center>;
  if (!user) return <Navigate to="/login" replace />;
  if (statusLoading) return <Center>{t.common.loading}</Center>;
  // Not "disabled": on a deployment without a key this route is not a place.
  if (!status || !status.enabled) return <Navigate to="/dashboard" replace />;

  // Which wall you hit decides what to do about it, so the two are never
  // collapsed into one "out of quota". Monthly wins when both are spent: it is
  // the one that is still there tomorrow.
  const blocked = status.remaining > 0 ? null : status.credits <= 0 ? "monthly" : "daily";
  const overLimit = prompt.length > MAX_PROMPT;
  const canSubmit =
    !busy && prompt.trim().length > 0 && !overLimit && !blocked && user.email_verified;

  return (
    <div className="min-h-screen flex flex-col bg-neutral-950">
      <Nav />
      <div className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 py-8">
        <h1 className="mb-1.5 flex items-center gap-2.5 text-lg font-brand text-neutral-100">
          <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-neutral-900 text-brand-400">
            <i className="fa-solid fa-wand-magic-sparkles text-sm" />
          </span>
          {t.generate.title}
        </h1>
        <p className="mb-5 text-xs text-neutral-600">{t.generate.subtitle}</p>

        {!user.email_verified && (
          <div className="mb-5 rounded-xl border border-amber-500/30 bg-amber-950/20 px-4 py-3 text-xs text-amber-200">
            <i className="fa-solid fa-triangle-exclamation mr-1.5" />
            {t.upload.emailUnverified}
            <Link to="/settings" className="ml-1.5 underline hover:text-amber-100">
              {t.upload.goVerify}
            </Link>
          </div>
        )}

        <div className="grid lg:grid-cols-3 gap-3">
          {/* Composer */}
          <div className="lg:col-span-2 rounded-2xl border border-neutral-800 bg-neutral-900/40 p-5">
            <div className="mb-1.5 flex items-baseline justify-between">
              <label htmlFor="ai-prompt" className="text-xs text-neutral-300">
                {t.generate.promptLabel}
              </label>
              <span
                className={`text-[10px] tabular-nums ${overLimit ? "text-red-400" : "text-neutral-600"}`}
              >
                {t.generate.promptCounter(prompt.length, MAX_PROMPT)}
              </span>
            </div>
            <textarea
              id="ai-prompt"
              ref={promptRef}
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              rows={5}
              placeholder={t.generate.promptPlaceholder}
              className={`w-full rounded-xl bg-neutral-950 border px-3 py-2.5 text-xs leading-relaxed outline-none transition resize-y placeholder-faint ${
                overLimit ? "border-red-500/60" : "border-neutral-800 focus:border-brand-500"
              }`}
            />

            <div className="mt-4">
              <div className="mb-1.5 text-[10px] uppercase tracking-wider text-neutral-600">
                {t.generate.sizeLabel}
              </div>
              <div className="flex flex-wrap gap-1.5">
                {sizes.map((s) => (
                  <button
                    key={s}
                    onClick={() => setSize(s)}
                    className={`inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-[11px] transition ${
                      s === activeSize
                        ? "border-brand-500 bg-brand-950/40 text-brand-300"
                        : "border-neutral-800 bg-neutral-900 text-neutral-400 hover:border-neutral-700 hover:text-neutral-200"
                    }`}
                  >
                    <RatioIcon ratio={s} active={s === activeSize} />
                    {s}
                  </button>
                ))}
              </div>
            </div>

            <div className="mt-3">
              <div className="mb-1.5 text-[10px] uppercase tracking-wider text-neutral-600">
                {t.generate.resolutionLabel}
              </div>
              <div className="flex flex-wrap gap-1.5">
                {resolutions.map((r) => (
                  <button
                    key={r}
                    onClick={() => setResolution(r)}
                    className={`rounded-lg border px-3 py-1.5 text-[11px] uppercase transition ${
                      r === activeResolution
                        ? "border-brand-500 bg-brand-950/40 text-brand-300"
                        : "border-neutral-800 bg-neutral-900 text-neutral-400 hover:border-neutral-700 hover:text-neutral-200"
                    }`}
                  >
                    {r}
                  </button>
                ))}
              </div>
            </div>

            {err && <div className="mt-3 text-[11px] text-red-400">{err}</div>}

            <div className="mt-4 flex items-center gap-3">
              <span className="text-[11px] text-neutral-600">{t.generate.costHint(1)}</span>
              <div className="flex-1" />
              <button
                onClick={submit}
                disabled={!canSubmit}
                className="rounded-xl bg-brand-600 px-5 py-2.5 text-sm font-medium text-brand-ink hover:bg-brand-500 disabled:bg-neutral-800 disabled:text-neutral-600 transition whitespace-nowrap"
              >
                {busy ? (
                  <>
                    <RingSpinner className="h-3.5 w-3.5 inline-block align-[-2px] mr-1.5" />
                    {t.generate.submitBusy}
                  </>
                ) : (
                  <>
                    <i className="fa-solid fa-wand-magic-sparkles mr-1.5 text-xs" />
                    {t.generate.submit}
                  </>
                )}
              </button>
            </div>
          </div>

          {/* Allowance */}
          <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-5">
            <div className="text-[10px] uppercase tracking-wider text-neutral-600 mb-3">
              {t.generate.quota.title}
            </div>
            <div className="text-3xl font-brand text-neutral-100 tabular-nums">
              {status.remaining}
              <span className="text-xs text-neutral-500 ml-1.5">
                {t.generate.quota.unit(status.remaining)}
              </span>
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

            {blocked === "daily" && (
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
            {blocked === "monthly" && (
              <div className="mt-4 rounded-xl border border-amber-500/30 bg-amber-950/20 px-3 py-2.5 text-[11px] text-amber-200">
                <div>
                  <i className="fa-solid fa-hourglass-end mr-1.5" />
                  {t.generate.quota.monthlyExhausted}
                </div>
                <div className="mt-1 text-amber-200/70">
                  {t.generate.quota.monthlyExhaustedHint}
                </div>
                <Link to="/space" className="mt-1.5 inline-block text-brand-400 hover:underline">
                  {t.generate.quota.checkinLink}
                </Link>
              </div>
            )}
            {/* Storage is the other thing a generated picture spends, and it is
                not on this card anywhere else. */}
            <div className="mt-4 flex items-center justify-between text-[11px]">
              <span className="text-neutral-600">{t.common.availableStorage}</span>
              <Link to="/space" className="text-neutral-300 hover:text-brand-300 transition">
                {formatBytes(user.available_bytes)}
              </Link>
            </div>
          </div>
        </div>

        {/* History */}
        <div className="mt-8 rounded-2xl border border-neutral-800 bg-neutral-900/40 overflow-hidden">
          <div className="flex items-center gap-2 px-5 py-3 border-b border-neutral-800/60">
            <span className="text-xs text-neutral-300">{t.generate.history.title}</span>
            {gens.length > 0 && (
              <span className="text-[10px] text-neutral-600">
                {t.generate.history.count(gens.length)}
              </span>
            )}
            {active && <RingSpinner className="h-3 w-3 text-brand-400" />}
          </div>

          {gens.length === 0 ? (
            <div className="px-5 py-14 text-center">
              <div className="inline-flex items-center justify-center w-14 h-14 rounded-2xl bg-neutral-900 mb-3">
                <i className="fa-solid fa-wand-magic-sparkles text-xl text-brand-500" />
              </div>
              <div className="text-sm text-neutral-400">{t.generate.history.empty}</div>
              <div className="mt-1 text-xs text-neutral-600">{t.generate.history.emptyHint}</div>
            </div>
          ) : (
            <div className="divide-y divide-neutral-800/50">
              {gens.map((g) => {
                const img = g.image_id ? images[g.image_id] : undefined;
                return (
                  <div key={g.id} className="flex items-start gap-3 px-5 py-3">
                    <Thumb
                      gen={g}
                      img={img}
                      onOpen={() => img && setDetail(img)}
                      openTitle={t.generate.history.openDetail}
                    />

                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <StatusBadge status={g.status} label={t.generate.history.status[g.status]} />
                        <span className="text-[10px] text-neutral-600">
                          {g.size} · {g.resolution.toUpperCase()}
                        </span>
                        <span className="text-[10px] text-faint">
                          {new Date(g.created_at).toLocaleString(locale)}
                        </span>
                      </div>

                      <div
                        className="mt-1 text-xs text-neutral-300 line-clamp-2 break-words"
                        title={g.prompt}
                      >
                        {g.prompt}
                      </div>

                      {(g.status === "pending" || g.status === "running") && (
                        <div className="mt-1 text-[10px] text-neutral-600">
                          {t.generate.history.working}
                        </div>
                      )}

                      {g.status === "failed" && (
                        <div className="mt-1.5 rounded-lg border border-red-500/25 bg-red-950/20 px-2.5 py-1.5 text-[10px] text-red-300">
                          <span className="text-red-400/70">{t.generate.history.failedLabel}: </span>
                          {g.error || t.generate.history.status.failed}
                          <span className="ml-1.5 text-neutral-500">
                            · {t.generate.history.refunded}
                          </span>
                        </div>
                      )}

                      <div className="mt-1.5 flex flex-wrap items-center gap-2">
                        <button
                          onClick={() => {
                            setPrompt(g.prompt);
                            promptRef.current?.focus();
                            window.scrollTo({ top: 0, behavior: "smooth" });
                          }}
                          title={t.generate.history.reusePrompt}
                          className="text-[10px] text-neutral-600 hover:text-brand-300 transition"
                        >
                          <i className="fa-solid fa-rotate-left mr-1 text-[9px]" />
                          {t.generate.history.reusePrompt}
                        </button>

                        {img && (
                          <>
                            <button
                              onClick={() => copyLink(img)}
                              className={`text-[10px] transition ${
                                copied === img.id
                                  ? "text-brand-300"
                                  : "text-neutral-600 hover:text-brand-300"
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
                              onClick={() => setDetail(img)}
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
      </div>

      {detail && (
        <ImageDetail
          img={detail}
          onClose={() => setDetail(null)}
          onDeleted={() => {
            // The generation record outlives its picture; drop the image so the
            // row stops offering links that now 404.
            setImages((prev) => {
              const next = { ...prev };
              delete next[detail.id];
              return next;
            });
            setDetail(null);
            refresh();
          }}
        />
      )}
      <Footer />
    </div>
  );
}

/**
 * The ratio drawn rather than only named. "3:2" and "2:3" are one character
 * apart and pick opposite orientations, which is a mistake worth making
 * impossible at a glance.
 */
function RatioIcon({ ratio, active }: { ratio: string; active: boolean }) {
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

function StatusBadge({ status, label }: { status: AIGeneration["status"]; label: string }) {
  const tone =
    status === "completed"
      ? "bg-brand-950/40 text-brand-300"
      : status === "failed"
        ? "bg-red-950/40 text-red-300"
        : "bg-neutral-800 text-neutral-400";
  return (
    <span className={`rounded-full px-2 py-0.5 text-[10px] ${tone}`}>
      {(status === "pending" || status === "running") && (
        <RingSpinner className="h-2.5 w-2.5 inline-block align-[-1px] mr-1" />
      )}
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
        <img
          src={img.thumb_url}
          alt={gen.prompt}
          loading="lazy"
          className="w-full h-full object-cover"
        />
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

function Center({ children }: { children: React.ReactNode }) {
  return <div className="min-h-screen flex items-center justify-center text-neutral-500">{children}</div>;
}
