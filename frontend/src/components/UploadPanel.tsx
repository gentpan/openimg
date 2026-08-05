import { useEffect, useState } from "react";
import { FORMAT_LABELS, linkFor, useUpload, type QueueItem } from "../UploadContext";
import { formatBytes } from "../api";
import { RingSpinner } from "./Spinner";

/**
 * Floating upload panel, bottom-right. Uploads keep running while you browse,
 * so the queue lives here rather than inline on the page that started it —
 * same reason a file manager puts transfers in a corner instead of a modal.
 *
 * It auto-expands when work arrives and collapses to a single bar once
 * everything finishes, so a finished queue stops competing for attention.
 */
export default function UploadPanel() {
  const {
    items,
    remove,
    clear,
    clearFinished,
    retry,
    format,
    setFormat,
    overallProgress,
    activeCount,
    doneCount,
    errorCount,
  } = useUpload();

  const [collapsed, setCollapsed] = useState(false);
  const [copied, setCopied] = useState<string | null>(null);
  const [wasActive, setWasActive] = useState(false);

  // Expand when a new batch starts; collapse once the last one lands.
  useEffect(() => {
    if (activeCount > 0 && !wasActive) {
      setCollapsed(false);
      setWasActive(true);
    } else if (activeCount === 0 && wasActive) {
      setWasActive(false);
    }
  }, [activeCount, wasActive]);

  if (items.length === 0) return null;

  const done = items.filter((i) => i.state === "done" && i.image);
  const allLinks = done.map((i) => linkFor(i.image!, format)).join("\n");
  const finished = activeCount === 0;

  async function copy(text: string, key: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(key);
      setTimeout(() => setCopied(null), 1500);
    } catch {}
  }

  return (
    <div className="fixed bottom-4 right-4 z-50 w-[min(22rem,calc(100vw-2rem))]">
      <div className="rounded-2xl border border-neutral-800 bg-neutral-900/95 shadow-panel overflow-hidden">
        {/* Header doubles as the collapsed state */}
        <button
          onClick={() => setCollapsed((c) => !c)}
          className="w-full flex items-center gap-2.5 px-3.5 py-2.5 hover:bg-neutral-800/40 transition text-left"
        >
          <span
            className={`shrink-0 w-6 h-6 rounded-lg flex items-center justify-center text-[10px] ${
              errorCount > 0
                ? "bg-red-900/40 text-red-300"
                : finished
                  ? "bg-teal-900/40 text-teal-300"
                  : "bg-brand-900/40 text-brand-300"
            }`}
          >
            {errorCount > 0 ? (
              <i className="fa-solid fa-triangle-exclamation" />
            ) : finished ? (
              <i className="fa-solid fa-check" />
            ) : (
              <RingSpinner className="h-3.5 w-3.5" />
            )}
          </span>

          <div className="flex-1 min-w-0">
            <div className="text-xs text-neutral-200">
              {finished
                ? errorCount > 0
                  ? `完成 ${doneCount} 张，${errorCount} 张失败`
                  : `已上传 ${doneCount} 张`
                : `上传中 ${doneCount}/${items.length}`}
            </div>
            {!finished && (
              <div className="mt-1 h-0.5 rounded-full bg-neutral-800 overflow-hidden">
                <div
                  className="h-full bg-brand-500 transition-all duration-300"
                  style={{ width: `${overallProgress}%` }}
                />
              </div>
            )}
          </div>

          <span className="shrink-0 text-[10px] text-neutral-500 tabular-nums">
            {!finished && `${overallProgress}%`}
          </span>
          <i
            className={`fa-solid ${collapsed ? "fa-chevron-up" : "fa-chevron-down"} text-[10px] text-neutral-600 shrink-0`}
          />
        </button>

        {!collapsed && (
          <>
            <div className="max-h-72 overflow-y-auto overscroll-contain border-t border-neutral-800/60">
              {items.map((item) => (
                <Row
                  key={item.id}
                  item={item}
                  copied={copied}
                  onCopy={copy}
                  onRemove={() => remove(item.id)}
                  onRetry={() => retry(item.id)}
                  link={item.image ? linkFor(item.image, format) : ""}
                />
              ))}
            </div>

            <div className="flex items-center gap-1.5 px-3 py-2 border-t border-neutral-800/60">
              {done.length > 0 && (
                <>
                  <div className="flex gap-0.5 rounded-lg bg-neutral-950 p-0.5">
                    {FORMAT_LABELS.map((f) => (
                      <button
                        key={f.key}
                        onClick={() => setFormat(f.key)}
                        className={`px-1.5 py-0.5 rounded-md text-[10px] transition ${
                          format === f.key ? "bg-brand-600 text-white" : "text-neutral-500 hover:text-neutral-100"
                        }`}
                      >
                        {f.label}
                      </button>
                    ))}
                  </div>
                  <button
                    onClick={() => copy(allLinks, "all")}
                    className="rounded-lg bg-brand-600 px-2.5 py-1 text-[10px] font-medium text-white hover:bg-brand-500 transition whitespace-nowrap"
                  >
                    <i className={`fa-solid ${copied === "all" ? "fa-check" : "fa-copy"} mr-1`} />
                    {copied === "all" ? "已复制" : `复制 ${done.length} 条`}
                  </button>
                </>
              )}
              <div className="flex-1" />
              {doneCount > 0 && activeCount > 0 && (
                <button
                  onClick={clearFinished}
                  className="text-[10px] text-neutral-600 hover:text-neutral-300 whitespace-nowrap"
                >
                  清除已完成
                </button>
              )}
              <button
                onClick={clear}
                title="清空队列"
                className="text-[10px] text-neutral-600 hover:text-red-400 whitespace-nowrap"
              >
                清空
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function Row({
  item,
  link,
  copied,
  onCopy,
  onRemove,
  onRetry,
}: {
  item: QueueItem;
  link: string;
  copied: string | null;
  onCopy: (text: string, key: string) => void;
  onRemove: () => void;
  onRetry: () => void;
}) {
  const saved =
    item.image && item.image.size_orig > item.image.size_stored
      ? item.image.size_orig - item.image.size_stored
      : 0;

  return (
    <div className="flex items-center gap-2.5 px-3 py-2 hover:bg-neutral-800/30 transition group">
      <div className="relative w-8 h-8 shrink-0 rounded-md overflow-hidden bg-neutral-800">
        {(item.image?.thumb_url || item.preview) && (
          <img
            src={item.image?.thumb_url || item.preview}
            alt=""
            decoding="async"
            className="w-full h-full object-cover"
          />
        )}
        {item.state === "uploading" && (
          <div className="absolute inset-0 bg-black/60 flex items-center justify-center">
            <span className="text-[9px] text-white tabular-nums">{item.progress}</span>
          </div>
        )}
        {item.state === "error" && (
          <div className="absolute inset-0 bg-red-950/70 flex items-center justify-center">
            <i className="fa-solid fa-xmark text-[10px] text-red-300" />
          </div>
        )}
      </div>

      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-1.5">
          <span className="text-[11px] text-neutral-300 truncate" title={item.file.name}>
            {item.file.name}
          </span>
          {item.deduplicated && (
            <span
              title="服务器上已有相同内容，直接秒传"
              className="shrink-0 rounded-full bg-teal-900/50 px-1 text-[9px] text-teal-300"
            >
              秒传
            </span>
          )}
        </div>

        {item.state === "done" ? (
          <div className="text-[10px] text-neutral-600 truncate">
            {formatBytes(item.file.size, 0)}
            {saved > 0 && <span className="text-accent-400"> · 省 {formatBytes(saved, 0)}</span>}
          </div>
        ) : item.state === "error" ? (
          <div className="text-[10px] text-red-400 truncate" title={item.error}>
            {item.error}
          </div>
        ) : (
          <div className="mt-1 h-0.5 rounded-full bg-neutral-800 overflow-hidden">
            <div
              className="h-full bg-brand-500 transition-all duration-300"
              style={{ width: `${item.state === "queued" ? 0 : item.progress}%` }}
            />
          </div>
        )}
      </div>

      <div className="flex items-center gap-0.5 shrink-0">
        {item.state === "done" && link && (
          <button
            onClick={() => onCopy(link, item.id)}
            title="复制链接"
            className={`w-6 h-6 rounded-md text-[10px] transition ${
              copied === item.id
                ? "bg-brand-600 text-white"
                : "text-neutral-500 hover:bg-neutral-800 hover:text-neutral-200"
            }`}
          >
            <i className={`fa-solid ${copied === item.id ? "fa-check" : "fa-copy"}`} />
          </button>
        )}
        {item.state === "error" && (
          <button
            onClick={onRetry}
            title="重试"
            className="w-6 h-6 rounded-md text-[10px] text-neutral-500 hover:bg-neutral-800 hover:text-neutral-200 transition"
          >
            <i className="fa-solid fa-rotate-right" />
          </button>
        )}
        <button
          onClick={onRemove}
          title="移除"
          className="w-6 h-6 rounded-md text-[10px] text-neutral-600 hover:bg-neutral-800 hover:text-neutral-300 transition opacity-0 group-hover:opacity-100"
        >
          <i className="fa-solid fa-xmark" />
        </button>
      </div>
    </div>
  );
}
