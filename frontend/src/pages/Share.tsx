import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { formatBytes, shareApi, type SharePayload } from "../api";
import Logo from "../Logo";
import { useTheme } from "../ThemeContext";
import { useToast } from "../ToastContext";
import { RingSpinner } from "../components/Spinner";

/**
 * The page behind a short link.
 *
 * Public, and built for a visitor who arrived from somewhere else entirely —
 * no navigation chrome, no account required, nothing that assumes they know
 * what this site is. The one job is: show the picture, hand over the link in
 * whatever format they need, and give them a way to report it.
 */

const REACTIONS: { kind: string; emoji: string; label: string }[] = [
  { kind: "like", emoji: "👍", label: "赞" },
  { kind: "clap", emoji: "👏", label: "鼓掌" },
  { kind: "party", emoji: "🎉", label: "庆祝" },
  { kind: "sparkle", emoji: "✨", label: "惊艳" },
  { kind: "smile", emoji: "🙂", label: "微笑" },
];

export default function SharePage() {
  const { code = "" } = useParams();
  const { theme, toggle } = useTheme();
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
      <Shell theme={theme} onToggle={toggle}>
        <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 py-20 text-center">
          <div className="mb-3 inline-flex h-14 w-14 items-center justify-center rounded-2xl bg-neutral-800">
            <i className="fa-solid fa-link-slash text-xl text-neutral-500" />
          </div>
          <div className="text-sm text-neutral-300">{err}</div>
          <Link to="/" className="mt-3 inline-block text-xs text-violet-400 hover:underline">
            前往 Openimg 首页 →
          </Link>
        </div>
      </Shell>
    );
  }

  if (!data) {
    return (
      <Shell theme={theme} onToggle={toggle}>
        <div className="py-24 text-center">
          <RingSpinner className="h-6 w-6 text-violet-400" />
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
    { label: "直链", value: data.url },
    { label: "短链", value: data.short_url },
    { label: "缩略图", value: data.thumb_url },
  ];

  return (
    <Shell theme={theme} onToggle={toggle} onReport={() => setReporting(true)}>
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
          <span className="rounded-md bg-violet-900/40 px-2 py-0.5 text-[10px] font-medium text-violet-300">
            {data.ext.toUpperCase()}
          </span>
          {data.views > 0 && <Meta icon="fa-eye" text={`${data.views} 次访问`} />}

          <div className="flex-1" />

          <Reactions code={code} data={data} onChange={setData} />

          <a
            href={data.url}
            download={data.orig_name}
            className="inline-flex h-8 items-center justify-center rounded-lg bg-violet-600 px-3 text-xs font-medium text-white transition hover:bg-violet-500"
          >
            <i className="fa-solid fa-download mr-1.5" />
            下载
          </a>
        </div>
      </div>

      <div className="mt-4 overflow-hidden rounded-2xl border border-neutral-800">
        {formats.map((f, i) => (
          <FormatRow key={f.label} label={f.label} value={f.value} first={i === 0} />
        ))}
      </div>

      <ShareRow url={data.short_url} title={data.orig_name} />

      <div className="mt-4 rounded-xl border-l-2 border-violet-500 bg-neutral-900/40 px-4 py-3 text-[11px] leading-relaxed text-neutral-500">
        <i className="fa-solid fa-circle-info mr-1.5 text-violet-400" />
        本站仅提供图片存储与分发服务，不拥有所展示内容的任何版权。所有图片版权归原作者或上传者所有。
        如发现内容侵犯您的合法权益，请点击右上角举报按钮提交投诉，我们将在收到通知后尽快核实并删除相关内容。
      </div>

      <div className="mt-5 flex flex-wrap items-center gap-2 border-t border-neutral-800/60 pt-4 text-[11px] text-neutral-600">
        <Link to="/" className="font-brand text-neutral-500 transition hover:text-violet-300">
          © {new Date().getFullYear()} Open<span className="text-violet-400">img</span>
        </Link>
        <div className="flex-1" />
        {data.uploader && <span>由 {data.uploader} 上传</span>}
        <span className="text-neutral-700">·</span>
        <span>{new Date(data.created_at).toLocaleString("zh-CN")}</span>
      </div>

      {reporting && <ReportDialog code={code} onClose={() => setReporting(false)} />}
    </Shell>
  );
}

function Shell({
  children,
  theme,
  onToggle,
  onReport,
}: {
  children: React.ReactNode;
  theme: string;
  onToggle: () => void;
  onReport?: () => void;
}) {
  return (
    <div className="min-h-screen bg-neutral-950">
      <div className="mx-auto w-full max-w-3xl px-4 py-5">
        <div className="mb-4 flex items-center gap-2.5">
          <a href="/" className="flex items-center gap-2.5">
            <Logo size={24} asLink={false} />
            <span className="font-brand text-base">
              Open<span className="text-violet-400">img</span>
            </span>
          </a>
          <div className="flex-1" />
          {onReport && (
            <button
              onClick={onReport}
              title="举报这张图片"
              className="inline-flex h-8 w-8 items-center justify-center rounded-lg bg-red-900/30 text-xs text-red-300 transition hover:bg-red-900/50"
            >
              <i className="fa-solid fa-flag" />
            </button>
          )}
          <button
            onClick={onToggle}
            title={theme === "dark" ? "切换到浅色主题" : "切换到深色主题"}
            aria-label={theme === "dark" ? "切换到浅色主题" : "切换到深色主题"}
            className="inline-flex h-8 w-8 items-center justify-center rounded-lg text-neutral-500 transition hover:bg-neutral-900 hover:text-violet-300"
          >
            <i className={`fa-solid ${theme === "dark" ? "fa-sun" : "fa-moon"} text-xs`} />
          </button>
        </div>
        {children}
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
  const [busy, setBusy] = useState(false);
  const toast = useToast();

  async function react(kind: string) {
    if (busy) return;
    setBusy(true);
    try {
      onChange(await shareApi.react(code, kind));
    } catch (e) {
      toast.error("操作失败", e instanceof Error ? e.message : undefined);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex items-center gap-0.5 rounded-full border border-neutral-800 bg-neutral-950/60 p-0.5">
      {REACTIONS.map((r) => {
        const n = data.reactions[r.kind] || 0;
        const mine = data.mine === r.kind;
        return (
          <button
            key={r.kind}
            onClick={() => react(r.kind)}
            disabled={busy}
            title={mine ? `取消${r.label}` : r.label}
            className={`inline-flex h-7 items-center gap-1 rounded-full px-2 text-sm transition disabled:opacity-60 ${
              mine ? "bg-violet-600/25 ring-1 ring-violet-500/40" : "hover:bg-neutral-800"
            }`}
          >
            <span>{r.emoji}</span>
            {n > 0 && (
              <span className={`text-[10px] tabular-nums ${mine ? "text-violet-300" : "text-neutral-500"}`}>
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
      <div className="flex w-32 shrink-0 items-center bg-violet-600 px-3 text-xs font-medium text-white sm:w-40">
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
        title="复制"
        className={`flex w-11 shrink-0 items-center justify-center text-xs transition ${
          copied ? "bg-violet-600 text-white" : "bg-neutral-900/40 text-neutral-500 hover:text-violet-300"
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
  const toast = useToast();
  const u = encodeURIComponent(url);
  const t = encodeURIComponent(title);

  const targets = [
    { icon: "fa-brands fa-x-twitter", label: "X", href: `https://x.com/intent/tweet?url=${u}&text=${t}` },
    { icon: "fa-brands fa-telegram", label: "Telegram", href: `https://t.me/share/url?url=${u}&text=${t}` },
    { icon: "fa-brands fa-facebook", label: "Facebook", href: `https://www.facebook.com/sharer/sharer.php?u=${u}` },
    { icon: "fa-brands fa-reddit-alien", label: "Reddit", href: `https://www.reddit.com/submit?url=${u}&title=${t}` },
    { icon: "fa-brands fa-weibo", label: "微博", href: `https://service.weibo.com/share/share.php?url=${u}&title=${t}` },
    { icon: "fa-solid fa-envelope", label: "邮件", href: `mailto:?subject=${t}&body=${u}` },
  ];

  return (
    <div className="mt-4 flex flex-wrap items-center justify-center gap-2">
      {targets.map((s) => (
        <a
          key={s.label}
          href={s.href}
          target="_blank"
          rel="noopener noreferrer"
          title={s.label}
          className="inline-flex h-9 w-9 items-center justify-center rounded-full border border-neutral-800 text-neutral-400 transition hover:border-violet-500 hover:text-violet-300"
        >
          <i className={`${s.icon} text-sm`} />
        </a>
      ))}
      <button
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(url);
            toast.success("短链已复制");
          } catch {}
        }}
        title="复制短链"
        className="inline-flex h-9 w-9 items-center justify-center rounded-full border border-neutral-800 text-neutral-400 transition hover:border-violet-500 hover:text-violet-300"
      >
        <i className="fa-solid fa-link text-sm" />
      </button>
    </div>
  );
}

function ReportDialog({ code, onClose }: { code: string; onClose: () => void }) {
  const toast = useToast();
  const [preset, setPreset] = useState<string | null>(null);
  const [reason, setReason] = useState("");
  const [contact, setContact] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const PRESETS = ["色情或血腥内容", "侵犯版权", "个人隐私信息", "诈骗或恶意链接", "其他"];
  const full = preset && preset !== "其他" ? `${preset}${reason ? " — " + reason : ""}` : reason;
  const ready = (full || "").trim().length >= 4;

  async function submit() {
    if (!ready || busy) return;
    setBusy(true);
    setErr(null);
    try {
      await shareApi.report(code, full!.trim(), contact.trim());
      onClose();
      toast.success("投诉已提交", "我们会尽快核实处理");
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center px-4">
      <div className="absolute inset-0 bg-black/50" onClick={() => !busy && onClose()} />
      <div className="shadow-panel relative w-full max-w-sm rounded-2xl border border-neutral-800 bg-neutral-900 p-5">
        <div className="mb-1 flex items-center gap-2">
          <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-red-900/40 text-xs text-red-300">
            <i className="fa-solid fa-flag" />
          </span>
          <span className="text-sm text-neutral-100">举报这张图片</span>
        </div>
        <p className="mb-3 text-[11px] leading-relaxed text-neutral-500">
          管理员会收到通知并核实。无需登录即可提交。
        </p>

        <div className="mb-3 flex flex-wrap gap-1.5">
          {PRESETS.map((p) => (
            <button
              key={p}
              onClick={() => setPreset(preset === p ? null : p)}
              className={`inline-flex h-8 items-center justify-center rounded-lg px-3 text-xs transition ${
                preset === p
                  ? "border border-red-500/40 bg-red-600/20 text-red-300"
                  : "border border-transparent bg-neutral-800 text-neutral-400 hover:text-neutral-100"
              }`}
            >
              {p}
            </button>
          ))}
        </div>

        <textarea
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          rows={3}
          maxLength={500}
          placeholder={preset && preset !== "其他" ? "补充说明（可选）" : "请说明问题，至少 4 个字"}
          className="placeholder-faint w-full resize-none rounded-lg border border-neutral-800 bg-neutral-950 px-2.5 py-2 text-xs outline-none focus:border-red-500"
        />
        <input
          value={contact}
          onChange={(e) => setContact(e.target.value)}
          placeholder="联系方式（可选）"
          className="placeholder-faint mt-2 h-8 w-full rounded-lg border border-neutral-800 bg-neutral-950 px-2.5 text-xs outline-none focus:border-red-500"
        />

        {err && <div className="mt-2 text-[11px] text-red-400">{err}</div>}

        <div className="mt-3 flex items-center gap-2">
          <div className="flex-1" />
          <button
            onClick={onClose}
            disabled={busy}
            className="inline-flex h-8 items-center justify-center rounded-lg px-3 text-xs text-neutral-400 transition hover:text-neutral-100"
          >
            取消
          </button>
          <button
            onClick={submit}
            disabled={!ready || busy}
            className="inline-flex h-8 items-center justify-center rounded-lg bg-red-600 px-3 text-xs font-medium text-white transition hover:bg-red-700 disabled:bg-neutral-800 disabled:text-neutral-500"
          >
            {busy ? <RingSpinner className="h-3.5 w-3.5" /> : "提交举报"}
          </button>
        </div>
      </div>
    </div>
  );
}
