import { useRef, useState } from "react";
import { Link, Navigate } from "react-router-dom";
import { useAuth } from "../AuthContext";
import Footer from "../Footer";
import Nav from "../components/Nav";
import ImageDetail from "../components/ImageDetail";
import { RingSpinner } from "../components/Spinner";
import { Center, GenerationHistory, OptionPicker, QuotaCard } from "../components/ai/parts";
import { MAX_PROMPT, useGenerations } from "../components/ai/generations";
import { aiApi } from "../api";
import { refreshAIStatus, useAIStatus } from "../aiStatus";
import { useLang } from "../LangContext";
import { useToast } from "../ToastContext";
import type { Image } from "../types";

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
 * the moment nothing is in flight, so an idle page is silent. That machinery,
 * the allowance card and the history list are shared with the retouch page —
 * see components/ai/shared; what stays here is the composer, which is the only
 * part the two pages do differently.
 */
export default function GeneratePage() {
  const { t } = useLang();
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

  const [detail, setDetail] = useState<Image | null>(null);

  const promptRef = useRef<HTMLTextAreaElement>(null);

  // The moment the last job settles: a completed picture consumed storage, and
  // a failed one handed a credit back. Both numbers live outside this page.
  const { gens, images, setImages, working, prepend } = useGenerations(!!user, "generate", () => {
    refresh();
    refreshAIStatus();
  });

  const sizes = status?.enabled ? status.sizes : [];
  const resolutions = status?.enabled ? status.resolutions : [];
  const activeSize = size && sizes.includes(size) ? size : (sizes[0] ?? "1:1");
  const activeResolution =
    resolution && resolutions.includes(resolution) ? resolution : (resolutions[0] ?? "1k");

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
      prepend(gen);
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

  if (loading) return <Center>{t.common.loading}</Center>;
  if (!user) return <Navigate to="/login" replace />;
  if (statusLoading) return <Center>{t.common.loading}</Center>;
  // Not "disabled": on a deployment without a key this route is not a place.
  if (!status || !status.enabled) return <Navigate to="/dashboard" replace />;

  const blocked = status.remaining <= 0;
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

        <GenerationHistory
          gens={gens}
          images={images}
          working={working}
          title={t.generate.history.title}
          empty={t.generate.history.empty}
          emptyHint={t.generate.history.emptyHint}
          icon="fa-wand-magic-sparkles"
          reuseLabel={t.generate.history.reusePrompt}
          onReuse={(g) => {
            setPrompt(g.prompt);
            promptRef.current?.focus();
            // 输入框在页面底部,所以是把它滚进视野,而不是回到顶部——顶部
            // 现在是历史列表,滚上去恰好看不到刚被填好的那个框。
            promptRef.current?.scrollIntoView({ behavior: "smooth", block: "center" });
          }}
          onOpenDetail={setDetail}
        />

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
              <OptionPicker
                label={t.generate.sizeLabel}
                options={sizes}
                value={activeSize}
                onChange={setSize}
                ratio
              />
            </div>

            <div className="mt-3">
              <OptionPicker
                label={t.generate.resolutionLabel}
                options={resolutions}
                value={activeResolution}
                onChange={setResolution}
                uppercase
              />
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

          <QuotaCard status={status} availableBytes={user.available_bytes} />
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
