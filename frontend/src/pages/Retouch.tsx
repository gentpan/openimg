import { useRef, useState } from "react";
import { Link, Navigate } from "react-router-dom";
import { useAuth } from "../AuthContext";
import Footer from "../Footer";
import Nav from "../components/Nav";
import ImageDetail from "../components/ImageDetail";
import { RingSpinner } from "../components/Spinner";
import ImagePicker from "../components/ai/ImagePicker";
import { Center, GenerationHistory, OptionPicker, QuotaCard } from "../components/ai/parts";
import { MAX_PROMPT, MAX_SOURCES, genSources, useGenerations } from "../components/ai/generations";
import { aiApi } from "../api";
import { refreshAIStatus, useAIStatus } from "../aiStatus";
import { useLang } from "../LangContext";
import { useToast } from "../ToastContext";
import type { Image } from "../types";

/**
 * One-click starting points.
 *
 * The prompt text is English and fixed in this file rather than in the
 * dictionary, because it is not interface copy — it is an instruction to the
 * model, and the model reads these five better in English than in either
 * language's translation of them. What is translated is the label on the
 * button. The text lands in the box editable, so it reads as a first draft
 * rather than a mode the user has switched into.
 */
const PRESETS = [
  {
    key: "watermark" as const,
    icon: "fa-eraser",
    prompt:
      "Remove all watermarks, logos and text overlays from this image. Keep everything else exactly unchanged.",
  },
  {
    key: "declutter" as const,
    icon: "fa-user-slash",
    prompt:
      "Remove the unwanted people and clutter in the background. Keep the main subject unchanged.",
  },
  {
    key: "background" as const,
    icon: "fa-image",
    prompt:
      "Replace the background with a clean solid studio backdrop. Keep the subject unchanged.",
  },
  {
    key: "restore" as const,
    icon: "fa-clock-rotate-left",
    prompt:
      "Restore this old photo: remove scratches and stains, fix fading, keep the original look.",
  },
  {
    key: "sharpen" as const,
    icon: "fa-wand-sparkles",
    prompt: "Enhance the sharpness and detail of this image without changing its content.",
  },
];

/**
 * AI retouching — the sister page to text-to-image.
 *
 * Same credits, same submission-then-poll shape, same allowance card and
 * history list (all of it in components/ai/shared). The one difference is what
 * goes in: one to four pictures the user already owns, chosen from their
 * gallery rather than uploaded. They are already on a public CDN origin, so the
 * server hands upstream their URLs — a local file picker here would upload a
 * second copy of a file this service is already serving.
 *
 * Like the generate page it only exists where the deployment configured an
 * upstream key; without one both this route and its nav entry are absent.
 */
export default function RetouchPage() {
  const { t } = useLang();
  const toast = useToast();
  const { user, loading, refresh } = useAuth();
  const { status, loading: statusLoading } = useAIStatus(!!user);

  const [prompt, setPrompt] = useState("");
  const [sources, setSources] = useState<Image[]>([]);
  const [picking, setPicking] = useState(false);
  // Null means "don't send it". Retouching is the case where that is a real
  // answer: with no size the output keeps the shape of the picture that went
  // in, which is what someone removing a watermark almost always wants.
  const [size, setSize] = useState<string | null>(null);
  const [resolution, setResolution] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const [detail, setDetail] = useState<Image | null>(null);

  const promptRef = useRef<HTMLTextAreaElement>(null);

  // Source pictures this session has seen, kept so a history row can show what
  // an edit started from. The generations endpoint only promises the *result*
  // image in its map — the sources are ordinary gallery images and may never
  // have been fetched on this page — so what the user picked is remembered here
  // rather than asked for again.
  const [known, setKnown] = useState<Record<string, Image>>({});

  const { gens, images, setImages, working, prepend } = useGenerations(!!user, "edit", () => {
    refresh();
    refreshAIStatus();
  });

  function remember(list: Image[]) {
    setKnown((prev) => {
      const next = { ...prev };
      for (const img of list) next[img.id] = img;
      return next;
    });
  }

  const resolveSource = (id: string): Image | undefined => known[id] ?? images[id];

  const sizes = status?.enabled ? status.sizes : [];
  const resolutions = status?.enabled ? status.resolutions : [];
  // Unlike the generate page there is no fallback to the first option: null is
  // a legitimate choice here, so the value is stored as picked and only
  // discarded when the server stops offering it.
  const activeSize = size && sizes.includes(size) ? size : null;
  // 清晰度没有"跟随原图"这一档:它是计费档位,留空等于让上游自己挑,而它
  // 可能挑最贵的 4k。所以和生成页一样落到第一档,界面显示的就是真的会用的值。
  const activeResolution =
    resolution && resolutions.includes(resolution) ? resolution : (resolutions[0] ?? "1k");

  async function submit() {
    const text = prompt.trim();
    if (!text || sources.length === 0 || busy) return;
    setBusy(true);
    setErr(null);
    try {
      const gen = await aiApi.edit(
        text,
        sources.map((s) => s.id),
        activeSize,
        activeResolution,
      );
      remember(sources);
      prepend(gen);
      toast.success(t.retouch.submitted, t.retouch.submittedDetail);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setErr(msg);
      toast.error(t.retouch.submitFailed, msg);
    } finally {
      refreshAIStatus();
      setBusy(false);
    }
  }

  if (loading) return <Center>{t.common.loading}</Center>;
  if (!user) return <Navigate to="/login" replace />;
  if (statusLoading) return <Center>{t.common.loading}</Center>;
  if (!status || !status.enabled) return <Navigate to="/dashboard" replace />;

  const blocked = status.remaining <= 0;
  const overLimit = prompt.length > MAX_PROMPT;
  const canSubmit =
    !busy &&
    prompt.trim().length > 0 &&
    !overLimit &&
    sources.length > 0 &&
    !blocked &&
    user.email_verified;

  return (
    <div className="min-h-screen flex flex-col bg-neutral-950">
      <Nav />
      <div className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 py-8">
        <h1 className="mb-1.5 flex items-center gap-2.5 text-lg font-brand text-neutral-100">
          <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-neutral-900 text-brand-400">
            <i className="fa-solid fa-wand-magic text-sm" />
          </span>
          {t.retouch.title}
        </h1>
        <p className="mb-5 text-xs text-neutral-600">{t.retouch.subtitle}</p>

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
          title={t.retouch.history.title}
          empty={t.retouch.history.empty}
          emptyHint={t.retouch.history.emptyHint}
          icon="fa-wand-magic"
          reuseLabel={t.retouch.history.reuse}
          onReuse={(g) => {
            setPrompt(g.prompt);
            // The sources come back too where they can: re-running an edit with
            // the same words on a different picture is rare, and re-picking the
            // same four out of a gallery is the tedious half of the job.
            const back = genSources(g)
              .map(resolveSource)
              .filter((i): i is Image => !!i);
            if (back.length > 0) setSources(back);
            promptRef.current?.focus();
            // 输入区在底部,滚它进视野而不是回顶部——顶部现在是历史列表。
            promptRef.current?.scrollIntoView({ behavior: "smooth", block: "center" });
          }}
          onOpenDetail={setDetail}
          resolveSource={resolveSource}
        />

        <div className="grid lg:grid-cols-3 gap-3">
          {/* Composer */}
          <div className="lg:col-span-2 rounded-2xl border border-neutral-800 bg-neutral-900/40 p-5">
            {/* Sources first: nothing below it means anything until there is a
                picture to apply it to. */}
            <div className="mb-1.5 flex items-baseline justify-between">
              <span className="text-xs text-neutral-300">{t.retouch.sourceLabel}</span>
              <span className="text-[10px] tabular-nums text-neutral-600">
                {t.retouch.sourceCount(sources.length, MAX_SOURCES)}
              </span>
            </div>

            {sources.length === 0 ? (
              <button
                onClick={() => setPicking(true)}
                className="flex w-full flex-col items-center justify-center gap-1.5 rounded-xl border border-dashed border-neutral-800 bg-neutral-950 px-4 py-8 text-neutral-500 transition hover:border-brand-500 hover:text-brand-300"
              >
                <i className="fa-solid fa-images text-lg" />
                <span className="text-xs">{t.retouch.sourcePick}</span>
                <span className="text-[10px] text-neutral-600">{t.retouch.sourceHint(MAX_SOURCES)}</span>
              </button>
            ) : (
              <div className="flex flex-wrap items-center gap-2">
                {sources.map((img, i) => (
                  <div
                    key={img.id}
                    className="group relative h-20 w-20 overflow-hidden rounded-xl border border-neutral-800"
                  >
                    <img
                      src={img.thumb_url}
                      alt={img.orig_name}
                      title={img.orig_name}
                      loading="lazy"
                      className="h-full w-full object-cover"
                    />
                    <span className="absolute left-1 top-1 flex h-4 w-4 items-center justify-center rounded-full bg-black/60 text-[9px] tabular-nums text-white">
                      {i + 1}
                    </span>
                    <button
                      onClick={() => setSources((prev) => prev.filter((p) => p.id !== img.id))}
                      title={t.common.remove}
                      aria-label={t.common.remove}
                      className="absolute right-1 top-1 flex h-4 w-4 items-center justify-center rounded-full bg-black/60 text-[9px] text-white/70 transition hover:bg-red-600 hover:text-white"
                    >
                      <i className="fa-solid fa-xmark" />
                    </button>
                  </div>
                ))}
                {sources.length < MAX_SOURCES && (
                  <button
                    onClick={() => setPicking(true)}
                    title={t.retouch.sourceAdd}
                    className="flex h-20 w-20 flex-col items-center justify-center gap-1 rounded-xl border border-dashed border-neutral-800 text-neutral-600 transition hover:border-brand-500 hover:text-brand-300"
                  >
                    <i className="fa-solid fa-plus text-sm" />
                    <span className="text-[10px]">{t.retouch.sourceAdd}</span>
                  </button>
                )}
              </div>
            )}

            {/* Presets. Five things people actually come here to do, each one a
                first draft in the box rather than a mode — the text stays
                editable, and the button does not stay lit. */}
            <div className="mt-4 mb-1.5 text-[10px] uppercase tracking-wider text-neutral-600">
              {t.retouch.presetsLabel}
            </div>
            <div className="flex flex-wrap gap-1.5">
              {PRESETS.map((p) => (
                <button
                  key={p.key}
                  onClick={() => {
                    setPrompt(p.prompt);
                    promptRef.current?.focus();
                  }}
                  className="inline-flex items-center gap-1.5 rounded-lg border border-neutral-800 bg-neutral-900 px-2.5 py-1.5 text-[11px] text-neutral-400 transition hover:border-neutral-700 hover:text-neutral-200"
                >
                  <i className={`fa-solid ${p.icon} text-[10px] text-neutral-600`} />
                  {t.retouch.presets[p.key]}
                </button>
              ))}
            </div>

            <div className="mt-4 mb-1.5 flex items-baseline justify-between">
              <label htmlFor="ai-edit-prompt" className="text-xs text-neutral-300">
                {t.retouch.promptLabel}
              </label>
              <span
                className={`text-[10px] tabular-nums ${overLimit ? "text-red-400" : "text-neutral-600"}`}
              >
                {t.generate.promptCounter(prompt.length, MAX_PROMPT)}
              </span>
            </div>
            <textarea
              id="ai-edit-prompt"
              ref={promptRef}
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              rows={4}
              placeholder={t.retouch.promptPlaceholder}
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
                autoLabel={t.retouch.matchSource}
                autoTitle={t.retouch.matchSourceHint}
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
              <span className="text-[11px] text-neutral-600">
                {sources.length === 0 ? t.retouch.needSource : t.generate.costHint(1)}
              </span>
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
                    <i className="fa-solid fa-wand-magic mr-1.5 text-xs" />
                    {t.retouch.submit}
                  </>
                )}
              </button>
            </div>
          </div>

          <QuotaCard status={status} availableBytes={user.available_bytes} />
        </div>
      </div>

      {picking && (
        <ImagePicker
          initial={sources}
          max={MAX_SOURCES}
          onCancel={() => setPicking(false)}
          onConfirm={(picked) => {
            setSources(picked);
            remember(picked);
            setPicking(false);
          }}
        />
      )}

      {detail && (
        <ImageDetail
          img={detail}
          onClose={() => setDetail(null)}
          onDeleted={() => {
            // The record outlives its picture. Drop it everywhere this page
            // could still offer a link to it: the result map, the remembered
            // sources, and the composer's own selection.
            setImages((prev) => {
              const next = { ...prev };
              delete next[detail.id];
              return next;
            });
            setKnown((prev) => {
              const next = { ...prev };
              delete next[detail.id];
              return next;
            });
            setSources((prev) => prev.filter((s) => s.id !== detail.id));
            setDetail(null);
            refresh();
          }}
        />
      )}
      <Footer />
    </div>
  );
}
