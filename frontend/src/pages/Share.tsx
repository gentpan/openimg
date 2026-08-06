import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { REPORT_CATEGORIES, formatBytes, shareApi, type SharePayload } from "../api";
import Logo from "../Logo";
import { useToast } from "../ToastContext";
import { RingSpinner } from "../components/Spinner";
import { useLang } from "../LangContext";
import type { Dict } from "../i18n";

/**
 * The page behind a short link.
 *
 * Public, and built for a visitor who arrived from somewhere else entirely —
 * no navigation chrome, no account required, nothing that assumes they know
 * what this site is. The one job is: show the picture, hand over the link in
 * whatever format they need, and give them a way to report it.
 */

// Keys resolved at render: a module constant is evaluated once at import,
// and the language can change at any point after.
const reactions = (t: Dict): { kind: string; emoji: string; label: string }[] => [
  { kind: "like", emoji: "👍", label: t.share.reaction.like },
  { kind: "clap", emoji: "👏", label: t.share.reaction.clap },
  { kind: "party", emoji: "🎉", label: t.share.reaction.party },
  { kind: "sparkle", emoji: "✨", label: t.share.reaction.sparkle },
  { kind: "smile", emoji: "🙂", label: t.share.reaction.smile },
];

export default function SharePage() {
  const { t } = useLang();
  const { locale } = useLang();
  const { code = "" } = useParams();
  
  const [data, setData] = useState<SharePayload | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [reporting, setReporting] = useState(false);

  useEffect(() => {
    shareApi
      .get(code)
      .then(setData)
      .catch((e) => setErr(e instanceof Error ? e.message : String(e)));
  }, [code]);

  if (err) {
    return (
      <Shell>
        <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 py-14 sm:py-20 text-center">
          <div className="mb-3 inline-flex h-14 w-14 items-center justify-center rounded-2xl bg-neutral-800">
            <i className="fa-solid fa-link-slash text-xl text-neutral-500" />
          </div>
          <div className="text-sm text-neutral-300">{err}</div>
          <Link to="/" className="mt-3 inline-block text-xs text-brand-400 hover:underline">
            {t.share.goHome}
          </Link>
        </div>
      </Shell>
    );
  }

  if (!data) {
    return (
      <Shell>
        <div className="py-24 text-center">
          <RingSpinner className="h-6 w-6 text-brand-400" />
        </div>
      </Shell>
    );
  }

  const formats = [
    { label: "BBCode", value: data.bbcode },
    { label: "BBCode w/ Link", value: data.bbcode_link },
    { label: "HTML", value: data.html },
    { label: "HTML w/ Link", value: data.html_link },
    { label: "HTML w/ Direct", value: data.html_direct },
    { label: "Markdown", value: data.markdown },
    { label: t.share.formatDirectLink, value: data.url },
    { label: t.common.shortLink, value: data.short_url },
    { label: t.share.formatThumbnail, value: data.thumb_url },
  ];

  return (
    <Shell onReport={() => setReporting(true)}
      footer={
        <>
          <Link to="/" className="font-brand text-neutral-500 transition hover:text-brand-300">
            © {new Date().getFullYear()} Open<span className="text-brand-400">img</span>
          </Link>
          <div className="flex-1" />
          {data.uploader && <span>由 {data.uploader} 上传</span>}
          <span className="text-neutral-700">·</span>
          <span>{new Date(data.created_at).toLocaleString(locale)}</span>
        </>
      }
    >
      <div className="overflow-hidden rounded-2xl border border-neutral-800 bg-neutral-900/40">
        <div className="flex items-center justify-center bg-neutral-950/60">
          <img
            src={data.url}
            alt={data.orig_name}
            className="max-h-[62vh] w-auto max-w-full object-contain"
          />
        </div>

        <div className="border-t border-neutral-800/60 px-4 py-3">
          <h1 className="truncate text-sm text-neutral-100" title={data.orig_name}>
            {data.orig_name}
          </h1>
        </div>

        <div className="flex flex-wrap items-center gap-3 border-t border-neutral-800/60 px-4 py-3">
          <Meta icon="fa-file" text={formatBytes(data.size)} />
          <Meta icon="fa-image" text={`${data.width} × ${data.height}`} />
          <span className="rounded-md bg-brand-900/40 px-2 py-0.5 text-[10px] font-medium text-brand-300">
            {data.ext.toUpperCase()}
          </span>
          {data.views > 0 && <Meta icon="fa-eye" text={`${data.views} 次访问`} />}

          <div className="flex-1" />

          <Reactions code={code} data={data} onChange={setData} />

          <a
            href={data.url}
            download={data.orig_name}
            className="inline-flex h-8 items-center justify-center rounded-lg bg-brand-600 px-3 text-xs font-medium text-brand-ink transition hover:bg-brand-500"
          >
            <i className="fa-solid fa-download mr-1.5" />
            {t.common.download}
          </a>
        </div>
      </div>

      <div className="mt-6 overflow-hidden rounded-2xl border border-neutral-800">
        {formats.map((f, i) => (
          <FormatRow key={f.label} label={f.label} value={f.value} first={i === 0} />
        ))}
      </div>

      <ShareRow url={data.short_url} title={data.orig_name} />

      <div className="mt-6 rounded-xl border-l-2 border-brand-500 bg-neutral-900/40 px-4 py-3 text-[11px] leading-relaxed text-neutral-500">
        <i className="fa-solid fa-circle-info mr-1.5 text-brand-400" />
        本站仅提供图片存储与分发服务，不拥有所展示内容的任何版权。所有图片版权归原作者或上传者所有。
        如发现内容侵犯您的合法权益，请点击右上角举报按钮提交投诉，我们将在收到通知后尽快核实并删除相关内容。
      </div>

      {reporting && <ReportDialog code={code} onClose={() => setReporting(false)} />}
    </Shell>
  );
}

function Shell({
  children,
  footer,
  onReport,
}: {
  children: React.ReactNode;
  footer?: React.ReactNode;
  onReport?: () => void;
}) {
  const { t } = useLang();
  return (
    <div className="flex min-h-screen flex-col bg-neutral-950">
      <div className="mx-auto flex w-full max-w-3xl flex-1 flex-col px-4 py-5">
        <div className="mb-6 flex items-center gap-2.5">
          <a href="/" className="flex items-center gap-2.5">
            <Logo size={24} asLink={false} />
            <span className="font-brand text-base">
              Open<span className="text-brand-400">img</span>
            </span>
          </a>
          <div className="flex-1" />
          {onReport && (
            <button
              onClick={onReport}
              title={t.report.imageTitle}
              className="inline-flex h-8 w-8 items-center justify-center rounded-lg bg-red-900/30 text-xs text-red-300 transition hover:bg-red-900/50"
            >
              <i className="fa-solid fa-flag" />
            </button>
          )}
        </div>

        <div>{children}</div>

        {/* mt-auto is what pins the last row to the bottom edge on a short
            page — a small image used to leave the footer floating mid-screen
            with dead space under it. On a tall page the auto margin collapses
            to nothing and pt-6 keeps it off the content above. */}
        {footer && (
          <div className="mt-auto pt-6">
            <div className="flex flex-wrap items-center gap-2 border-t border-neutral-800/60 pt-4 text-[11px] text-neutral-600">
              {footer}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function Meta({ icon, text }: { icon: string; text: string }) {
  return (
    <span className="inline-flex items-center gap-1.5 text-xs text-neutral-400">
      <i className={`fa-solid ${icon} text-[10px] text-neutral-600`} />
      {text}
    </span>
  );
}

/** Emoji responses. Optimistic: the count moves on click and the server's
 *  answer replaces it, so a slow network doesn't make the button feel dead. */
function Reactions({
  code,
  data,
  onChange,
}: {
  code: string;
  data: SharePayload;
  onChange: (d: SharePayload) => void;
}) {
  const { t } = useLang();
  const [busy, setBusy] = useState(false);
  const toast = useToast();

  async function react(kind: string) {
    if (busy) return;
    setBusy(true);
    try {
      onChange(await shareApi.react(code, kind));
    } catch (e) {
      toast.error(t.share.reaction.failed, e instanceof Error ? e.message : undefined);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex items-center gap-0.5 rounded-full border border-neutral-800 bg-neutral-950/60 p-0.5">
      {reactions(t).map((r) => {
        const n = data.reactions[r.kind] || 0;
        const mine = data.mine === r.kind;
        return (
          <button
            key={r.kind}
            onClick={() => react(r.kind)}
            disabled={busy}
            title={mine ? `取消${r.label}` : r.label}
            className={`inline-flex h-7 items-center gap-1 rounded-full px-2 text-sm transition disabled:opacity-60 ${
              mine ? "bg-brand-600/25 ring-1 ring-brand-500/40" : "hover:bg-neutral-800"
            }`}
          >
            <span>{r.emoji}</span>
            {n > 0 && (
              <span className={`text-[10px] tabular-nums ${mine ? "text-brand-300" : "text-neutral-500"}`}>
                {n}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}

function FormatRow({ label, value, first }: { label: string; value: string; first: boolean }) {
  const { t } = useLang();
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {}
  }

  return (
    <div className={`flex items-stretch ${first ? "" : "border-t border-neutral-800/60"}`}>
      <div className="flex w-32 shrink-0 items-center bg-brand-600 px-3 text-xs font-medium text-brand-ink sm:w-40">
        {label}
      </div>
      <input
        readOnly
        value={value}
        onFocus={(e) => e.currentTarget.select()}
        className="min-w-0 flex-1 bg-neutral-900/40 px-3 py-2.5 font-mono text-[11px] text-neutral-300 outline-none"
      />
      <button
        onClick={copy}
        title={t.common.copy}
        className={`flex w-11 shrink-0 items-center justify-center text-xs transition ${
          copied ? "bg-brand-600 text-brand-ink" : "bg-neutral-900/40 text-neutral-500 hover:text-brand-300"
        }`}
      >
        <i className={`fa-solid ${copied ? "fa-check" : "fa-copy"}`} />
      </button>
    </div>
  );
}

/** Share targets. Each opens the network's own composer with the short link
 *  prefilled — no SDKs, no third-party scripts, nothing that would let another
 *  site watch who views an image here. */
function ShareRow({ url, title }: { url: string; title: string }) {
  const { t } = useLang();
  const toast = useToast();
  const u = encodeURIComponent(url);
  const encodedTitle = encodeURIComponent(title);

  const targets = [
    { icon: "fa-brands fa-x-twitter", label: "X", href: `https://x.com/intent/tweet?url=${u}&text=${encodedTitle}` },
    { icon: "fa-brands fa-telegram", label: "Telegram", href: `https://t.me/share/url?url=${u}&text=${encodedTitle}` },
    { icon: "fa-brands fa-facebook", label: "Facebook", href: `https://www.facebook.com/sharer/sharer.php?u=${u}` },
    { icon: "fa-brands fa-reddit-alien", label: "Reddit", href: `https://www.reddit.com/submit?url=${u}&title=${encodedTitle}` },
    { icon: "fa-brands fa-weibo", label: t.share.targetWeibo, href: `https://service.weibo.com/share/share.php?url=${u}&title=${encodedTitle}` },
    { icon: "fa-solid fa-envelope", label: t.share.targetEmail, href: `mailto:?subject=${encodedTitle}&body=${u}` },
  ];

  return (
    <div className="mt-6 flex flex-wrap items-center justify-center gap-2">
      {targets.map((s) => (
        <a
          key={s.label}
          href={s.href}
          target="_blank"
          rel="noopener noreferrer"
          title={s.label}
          className="inline-flex h-9 w-9 items-center justify-center rounded-full border border-neutral-800 text-neutral-400 transition hover:border-brand-500 hover:text-brand-300"
        >
          <i className={`${s.icon} text-sm`} />
        </a>
      ))}
      <button
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(url);
            toast.success(t.share.shortLinkCopied);
          } catch {}
        }}
        title={t.share.copyShortLink}
        className="inline-flex h-9 w-9 items-center justify-center rounded-full border border-neutral-800 text-neutral-400 transition hover:border-brand-500 hover:text-brand-300"
      >
        <i className="fa-solid fa-link text-sm" />
      </button>
    </div>
  );
}

function ReportDialog({ code, onClose }: { code: string; onClose: () => void }) {
  const { t } = useLang();
  const toast = useToast();
  const [category, setCategory] = useState<string | null>(null);
  const [reason, setReason] = useState("");
  const [contact, setContact] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  // A type is required; the description is not. The category is what gets
  // triaged, and demanding prose on top of it only teaches people to type
  // filler to get past the button.
  const ready = !!category;

  async function submit() {
    if (!ready || busy) return;
    setBusy(true);
    setErr(null);
    try {
      await shareApi.report(code, category!, reason.trim(), contact.trim());
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
      <div className="shadow-panel relative flex max-h-[88vh] w-full max-w-md flex-col overflow-hidden rounded-2xl border border-neutral-800 bg-neutral-900">
        <div className="flex items-center gap-2 border-b border-neutral-800/60 px-5 py-3.5">
          <span className="text-sm text-neutral-100">{t.report.dialogTitle}</span>
          <div className="flex-1" />
          <button
            onClick={onClose}
            disabled={busy}
            aria-label={t.common.close}
            className="inline-flex h-7 w-7 items-center justify-center rounded-lg text-neutral-500 transition hover:bg-neutral-800 hover:text-neutral-200"
          >
            <i className="fa-solid fa-xmark" />
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          <div className="mb-2 text-xs text-neutral-300">{t.report.categoryLabel}</div>
          <div className="space-y-2">
            {REPORT_CATEGORIES.map((c) => {
              const on = category === c.key;
              return (
                <button
                  key={c.key}
                  onClick={() => setCategory(c.key)}
                  className={`flex w-full items-center gap-2.5 rounded-lg border px-3 py-2.5 text-left text-xs transition ${
                    on
                      ? "border-brand-500/50 bg-brand-600/15 text-brand-200"
                      : "border-neutral-800 bg-neutral-950/40 text-neutral-300 hover:border-neutral-700"
                  }`}
                >
                  <span
                    className={`flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full border ${
                      on ? "border-brand-400" : "border-neutral-600"
                    }`}
                  >
                    {on && <span className="h-1.5 w-1.5 rounded-full bg-brand-400" />}
                  </span>
                  {c.label}
                </button>
              );
            })}
          </div>

          <div className="mb-1.5 mt-4 text-xs text-neutral-300">
            {t.report.detailsLabel} <span className="text-neutral-600">{t.common.optional}</span>
          </div>
          <textarea
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={3}
            maxLength={500}
            placeholder={t.report.detailsPlaceholder}
            className="placeholder-faint w-full resize-none rounded-lg border border-neutral-800 bg-neutral-950 px-3 py-2 text-xs outline-none focus:border-brand-500"
          />

          <input
            value={contact}
            onChange={(e) => setContact(e.target.value)}
            placeholder={t.report.contactPlaceholder}
            className="placeholder-faint mt-2 h-8 w-full rounded-lg border border-neutral-800 bg-neutral-950 px-3 text-xs outline-none focus:border-brand-500"
          />

          {err && <div className="mt-2 text-[11px] text-red-400">{err}</div>}
        </div>

        <div className="flex items-center gap-2 border-t border-neutral-800/60 px-5 py-3">
          <button
            onClick={onClose}
            disabled={busy}
            className="inline-flex h-8 items-center justify-center rounded-lg border border-neutral-800 px-4 text-xs text-neutral-300 transition hover:border-neutral-700"
          >
            {t.common.cancel}
          </button>
          <button
            onClick={submit}
            disabled={!ready || busy}
            className="inline-flex h-8 items-center justify-center rounded-lg bg-red-600 px-4 text-xs font-medium text-white transition hover:bg-red-700 disabled:bg-neutral-800 disabled:text-neutral-500"
          >
            {busy ? <RingSpinner className="h-3.5 w-3.5" /> : t.report.submit}
          </button>
          <div className="flex-1" />
          <span className="text-[10px] text-faint">{t.report.noLogin}</span>
        </div>
      </div>
    </div>
  );
}
