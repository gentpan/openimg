import Icon from "../Icon";
import { useRef, useState } from "react";
import { Link, Navigate } from "react-router-dom";
import { useAuth } from "../AuthContext";
import Footer from "../Footer";
import Nav from "../components/Nav";
import ImageDetail from "../components/ImageDetail";
import { RingSpinner } from "../components/Spinner";
import SourceImages from "../components/ai/SourceImages";
import {
  Center,
  GenerationHistory,
  OptionPicker,
  QuotaInline,
  QuotaNotice,
} from "../components/ai/parts";
import {
  MAX_PROMPT,
  MAX_SOURCES,
  canSpend,
  genSources,
  useGenerations,
  useKnownImages,
} from "../components/ai/generations";
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
 * model, and the model reads these better in English than in either language's
 * translation of them. What is translated is the label on the button. The text
 * lands in the box editable, so it reads as a first draft rather than a mode
 * the user has switched into.
 *
 * Every sentence ends by naming what must not change. Without that the model
 * treats the request as licence to repaint the whole picture, and someone who
 * asked for a watermark to go gets a different photograph back. Two of them
 * carry a further constraint that is easy to mistake for padding: the portrait
 * one forbids reshaping the face (skin retouching slides into a new person
 * otherwise), and the extend one forbids touching what is already there
 * (otherwise "extend" quietly becomes "redraw, wider").
 *
 * 这 11 条与 macOS 端 RetouchPrompts 逐字相同,改这里要连那边一起改——同一个
 * 按钮在两端发出不同的指令,就是同一个按钮出两种图。
 */
const PRESETS = [
  {
    key: "watermark" as const,
    icon: "eraser",
    prompt:
      "Remove all watermarks, logos and text overlays from this image. Keep everything else exactly unchanged.",
  },
  {
    key: "declutter" as const,
    icon: "user-slash",
    prompt:
      "Remove the unwanted people and clutter in the background. Keep the main subject unchanged.",
  },
  {
    key: "background" as const,
    icon: "image",
    prompt:
      "Replace the background with a clean solid studio backdrop. Keep the subject unchanged.",
  },
  {
    key: "restore" as const,
    icon: "clock-rotate-left",
    prompt:
      "Restore this old photo: remove scratches and stains, fix fading, keep the original look.",
  },
  {
    key: "sharpen" as const,
    icon: "wand-sparkles",
    prompt: "Enhance the sharpness and detail of this image without changing its content.",
  },
  {
    key: "text" as const,
    icon: "text-slash",
    prompt:
      "Remove all text, captions and subtitles from this image. Reconstruct what was behind them; keep everything else unchanged.",
  },
  {
    key: "whitebg" as const,
    icon: "square",
    prompt:
      "Cut out the product and place it on a pure white background, centered, with soft even lighting and a natural contact shadow.",
  },
  {
    key: "colorize" as const,
    icon: "palette",
    prompt:
      "Colorize this black and white photo with natural, period-accurate colors. Keep the original composition and details.",
  },
  {
    key: "portrait" as const,
    icon: "face-smile",
    prompt:
      "Retouch this portrait: even out skin tone, remove blemishes, brighten the eyes. Keep the person clearly recognizable — do not reshape the face.",
  },
  {
    key: "cartoon" as const,
    icon: "paintbrush",
    prompt:
      "Redraw this image as a clean flat illustration with bold outlines and simple shading. Keep the subject and composition recognizable.",
  },
  {
    key: "expand" as const,
    icon: "expand",
    prompt:
      "Extend this image outward, continuing the scene naturally on all sides. Keep the existing content untouched.",
  },
];

/**
 * AI retouching — the sister page to text-to-image.
 *
 * Same credits, same submission-then-poll shape, same allowance readout and
 * history list (all of it in components/ai/shared). The one difference is what
 * goes in: one to four pictures out of the user's own gallery. Dropping a local
 * file on the source row is a shortcut to the same place rather than a second
 * intake — it uploads through the ordinary pipeline first and the resulting
 * gallery picture is what gets selected, so the server still hands upstream a
 * URL it can fetch and the record still names an image that can be clicked
 * back into. All of that lives in components/ai/SourceImages, shared with the
 * generate page's reference pictures.
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
  // Null means "don't send it". Retouching is the case where that is a real
  // answer: with no size the output keeps the shape of the picture that went
  // in, which is what someone removing a watermark almost always wants.
  const [size, setSize] = useState<string | null>(null);
  const [resolution, setResolution] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const [detail, setDetail] = useState<Image | null>(null);

  const promptRef = useRef<HTMLTextAreaElement>(null);

  const { gens, images, setImages, working, prepend, remove } = useGenerations(!!user, "edit", () => {
    refresh();
    refreshAIStatus();
  });

  // What the user picked or uploaded, kept so a history row can show what an
  // edit started from.
  const { remember, forget, resolve: resolveSource } = useKnownImages(images);

  /** Selection and its bookkeeping move together — a source that is not
   *  remembered is a history row that cannot draw its own thumbnail. */
  function pickSources(next: Image[]) {
    setSources(next);
    remember(next);
  }

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

  // 本地额度见底不等于不能生成:关联了 pic.bi 且那边还有钱时,后端会自动
  // 接管。这个判断只有一处(canSpend),额度提示条也问同一处——两边各算一遍
  // 迟早会算出两个答案,变成"按钮是灰的,旁边写着还有余额"。
  const blocked = !canSpend(status);
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
            <Icon name="wand-magic" className="text-sm"  />
          </span>
          {t.retouch.title}
        </h1>
        <p className="mb-5 text-xs text-neutral-600">{t.retouch.subtitle}</p>

        {!user.email_verified && (
          <div className="mb-5 rounded-xl border border-amber-500/30 bg-amber-950/20 px-4 py-3 text-xs text-amber-200">
            <Icon name="triangle-exclamation" className="mr-1.5"  />
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
          icon="wand-magic"
          reuseLabel={t.retouch.history.reuse}
          onDelete={(g, alsoImage) => {
            remove(g, alsoImage)
              .then(() => toast.success(t.generate.history.removed))
              .catch((e) => toast.error(String(e)));
          }}
          onReuse={(g) => {
            setPrompt(g.prompt);
            // The sources come back too where they can: re-running an edit with
            // the same words on a different picture is rare, and re-picking the
            // same four out of a gallery is the tedious half of the job.
            const back = genSources(g)
              .map((id) => resolveSource(id))
              .filter((i): i is Image => !!i);
            if (back.length > 0) setSources(back);
            promptRef.current?.focus();
            // 输入区在底部,滚它进视野而不是回顶部——顶部现在是历史列表。
            promptRef.current?.scrollIntoView({ behavior: "smooth", block: "center" });
          }}
          onOpenDetail={setDetail}
          resolveSource={resolveSource}
        />

        {/* 用完了才出现,平时不占版面——数字已经在输入区底排说清了。 */}
        <QuotaNotice status={status} />

        {/* Composer */}
        <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-5">
          {/* Sources first: nothing below it means anything until there is a
              picture to apply it to. */}
          <SourceImages
            value={sources}
            onChange={pickSources}
            max={MAX_SOURCES}
            variant="required"
            // An upload spent storage; the figure in the nav is the one that
            // has to move.
            onUploaded={() => refresh()}
          />

          {/* Presets: the eleven things people actually come here to do, each
              one a first draft in the box rather than a mode — the text stays
              editable after it lands.

              Unselected chips carry a visible border. Without one they are a
              row of grey words on a grey panel, and the thing a first-time
              visitor most needs to discover here is that they are buttons at
              all. The chosen one fills with the brand colour rather than
              tinting, so at a glance it is obvious which sentence is currently
              in the box — and clicking it again empties the box, because the
              alternative for a mis-click is selecting the text by hand. */}
          <div className="mt-4 mb-1.5 text-[10px] uppercase tracking-wider text-neutral-600">
            {t.retouch.presetsLabel}
          </div>
          <div className="flex flex-wrap gap-1.5">
            {PRESETS.map((p) => {
              // Compared against the box, not stored: editing the text after
              // picking a preset means it is no longer that preset, and the
              // chip should stop claiming otherwise.
              const active = prompt === p.prompt;
              return (
                <button
                  key={p.key}
                  title={active ? t.retouch.presetClear : undefined}
                  onClick={() => {
                    setPrompt(active ? "" : p.prompt);
                    promptRef.current?.focus();
                  }}
                  className={`inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-[11px] transition ${
                    active
                      ? "border-brand-600 bg-brand-600 font-medium text-brand-ink"
                      : "border-neutral-700 bg-neutral-900 text-neutral-400 hover:border-neutral-600 hover:text-neutral-200"
                  }`}
                >
                  <Icon
                    name={p.icon}
                    className={`text-[10px] ${
                      active ? "text-brand-ink/70" : "text-neutral-600"
                    }`}
                  />
                  {t.retouch.presets[p.key]}
                </button>
              );
            })}
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

          <div className="mt-4 flex flex-wrap items-center gap-x-3 gap-y-2">
            <span className="text-[11px] text-neutral-600">
              {sources.length === 0 ? t.retouch.needSource : t.generate.costHint(1)}
            </span>
            <div className="flex-1" />
            {/* 底排本来空着一大片,而「还剩几次」正是按下按钮之前最后要确认
                的事——放在按钮边上比放在旁边一张卡里更该看见。 */}
            <QuotaInline status={status} />
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
                  <Icon name="wand-magic" className="mr-1.5 text-xs"  />
                  {t.retouch.submit}
                </>
              )}
            </button>
          </div>
        </div>
      </div>

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
            forget(detail.id);
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
