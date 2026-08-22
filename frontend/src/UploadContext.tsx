import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useAuth } from "./AuthContext";
import { useLang } from "./LangContext";
import { useToast } from "./ToastContext";
import { formatBytes, imageApi } from "./api";
import type { Image } from "./types";

export type ItemState = "queued" | "uploading" | "done" | "error";

export interface QueueItem {
  id: string;
  /** 本地文件。按网址取图时没有——那些字节从来没有经过浏览器。 */
  file?: File;
  /** 按网址取图时的来源地址。 */
  sourceURL?: string;
  /** 显示名。两条来源都有,免得每个用到的地方都要判一次 file 在不在。 */
  name: string;
  /** 字节数;网址取图时事先不知道,为 0。 */
  size: number;
  preview: string;
  state: ItemState;
  progress: number;
  image?: Image;
  deduplicated?: boolean;
  error?: string;
}

export type LinkFormat = "url" | "markdown" | "html" | "bbcode";

export const FORMAT_LABELS: { key: LinkFormat; label: string }[] = [
  { key: "url", label: "URL" },
  { key: "markdown", label: "MD" },
  { key: "html", label: "HTML" },
  { key: "bbcode", label: "BB" },
];

export function linkFor(img: Image, fmt: LinkFormat): string {
  switch (fmt) {
    case "markdown":
      return img.markdown;
    case "html":
      return img.html;
    case "bbcode":
      return img.bbcode;
    default:
      return img.url;
  }
}

// Uploads run a few at a time: the server transcodes synchronously, so firing
// twenty at once just queues them behind each other while holding twenty
// connections open.
const CONCURRENCY = 3;

/** Wide enough to stay crisp in the 32px slot on a 2x display. */
const PREVIEW_PX = 96;

/**
 * Builds a preview sized for the slot it renders into, not for the file.
 *
 * `URL.createObjectURL(file)` is one line and looks free, but the browser still
 * has to decode the *original* to paint that 32px square: a 12 MP photo becomes
 * a 48 MB bitmap. Twenty-five of those blow past the decoded-image cache, so
 * every scroll frame evicts and re-decodes — which is exactly when the panel
 * stutters, and why it only shows up once the queue gets long.
 *
 * createImageBitmap decodes and downscales in one step without the full-size
 * bitmap ever reaching the DOM, so each row costs a few KB instead of tens of MB.
 */
async function makePreview(file: File): Promise<string> {
  try {
    const bmp = await createImageBitmap(file, { resizeWidth: PREVIEW_PX, resizeQuality: "low" });
    const canvas = document.createElement("canvas");
    canvas.width = bmp.width;
    canvas.height = bmp.height;
    canvas.getContext("2d")?.drawImage(bmp, 0, 0);
    bmp.close();
    const blob = await new Promise<Blob | null>((res) => canvas.toBlob(res, "image/webp", 0.7));
    if (blob) return URL.createObjectURL(blob);
  } catch {
    // SVG, HEIC, or anything this browser can't decode natively — fall back to
    // the original rather than showing a blank square.
  }
  return URL.createObjectURL(file);
}

interface UploadCtx {
  items: QueueItem[];
  enqueue: (files: FileList | File[]) => void;
  enqueueURL: (url: string) => void;
  remove: (id: string) => void;
  clear: () => void;
  clearFinished: () => void;
  retry: (id: string) => void;
  format: LinkFormat;
  setFormat: (f: LinkFormat) => void;
  /** 0-100 across the whole queue, weighted by file size. */
  overallProgress: number;
  activeCount: number;
  doneCount: number;
  errorCount: number;
}

const Ctx = createContext<UploadCtx | null>(null);

/**
 * Holds the upload queue for the whole app, so navigating away from the upload
 * page doesn't abandon in-flight uploads — the floating panel follows you.
 */
export function UploadProvider({ children }: { children: ReactNode }) {
  const { user, refresh } = useAuth();
  const { t } = useLang();
  const toast = useToast();
  const [items, setItems] = useState<QueueItem[]>([]);
  const [format, setFormat] = useState<LinkFormat>("url");
  const runningRef = useRef(0);

  // Lets the async preview check whether its item still exists without doing
  // side effects inside a state updater, which React may invoke twice.
  const itemsRef = useRef<QueueItem[]>([]);
  useEffect(() => {
    itemsRef.current = items;
  }, [items]);

  const enqueue = useCallback((files: FileList | File[]) => {
    const picked = Array.from(files).filter((f) => f.type.startsWith("image/"));
    if (picked.length === 0) return;
    const added = picked.map((file) => ({
      id: `${file.name}-${file.size}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      file,
      name: file.name,
      size: file.size,
      preview: "",
      state: "queued" as ItemState,
      progress: 0,
    }));
    // Queued synchronously so dropping 30 files doesn't wait on 30 decodes;
    // each preview is patched in when it lands.
    setItems((prev) => [...prev, ...added]);
    for (const item of added) {
      if (!item.file) continue;
      void makePreview(item.file).then((url) => {
        if (!itemsRef.current.some((i) => i.id === item.id)) {
          URL.revokeObjectURL(url); // removed while we were decoding
          return;
        }
        setItems((prev) => prev.map((i) => (i.id === item.id ? { ...i, preview: url } : i)));
      });
    }
  }, []);

  /**
   * 按网址取图。
   *
   * 排进同一条队列,而不是另开一份状态:取回来之后它和别的图完全一样——同样要
   * 显示结果、复制链接、去重提示。两份队列意味着这些都要写两遍,而写两遍的东
   * 西迟早会不一致。
   */
  const enqueueURL = useCallback((raw: string) => {
    const url = raw.trim();
    if (!url) return;
    let label = url;
    try {
      const u = new URL(url);
      label = u.pathname.split("/").filter(Boolean).pop() || u.hostname;
    } catch {
      // 解析不了就原样显示,校验交给服务端——前端再判一遍只会和后端的规则
      // 慢慢分叉。
    }
    setItems((prev) => [
      ...prev,
      {
        id: `url-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
        sourceURL: url,
        name: label,
        size: 0,
        preview: "",
        state: "queued" as ItemState,
        progress: 0,
      },
    ]);
  }, []);

  const remove = useCallback((id: string) => {
    setItems((prev) => {
      const target = prev.find((i) => i.id === id);
      if (target) URL.revokeObjectURL(target.preview);
      return prev.filter((i) => i.id !== id);
    });
  }, []);

  const clear = useCallback(() => {
    setItems((prev) => {
      prev.forEach((i) => URL.revokeObjectURL(i.preview));
      return [];
    });
  }, []);

  const clearFinished = useCallback(() => {
    setItems((prev) => {
      prev.filter((i) => i.state === "done").forEach((i) => URL.revokeObjectURL(i.preview));
      return prev.filter((i) => i.state !== "done");
    });
  }, []);

  const retry = useCallback((id: string) => {
    setItems((prev) =>
      prev.map((i) => (i.id === id ? { ...i, state: "queued", progress: 0, error: undefined } : i)),
    );
  }, []);

  // Pump the queue, keeping CONCURRENCY uploads in flight.
  useEffect(() => {
    if (!user) return;
    const next = items.find((i) => i.state === "queued");
    if (!next || runningRef.current >= CONCURRENCY) return;

    runningRef.current++;
    setItems((prev) => prev.map((i) => (i.id === next.id ? { ...i, state: "uploading" } : i)));

    const run = next.sourceURL
      ? imageApi.uploadFromURL(next.sourceURL)
      : imageApi.upload(next.file!, {
          onProgress: (pct) =>
            setItems((prev) => prev.map((i) => (i.id === next.id ? { ...i, progress: pct } : i))),
        });

    run
      .then((res) => {
        setItems((prev) =>
          prev.map((i) =>
            i.id === next.id
              ? { ...i, state: "done", progress: 100, image: res.image, deduplicated: res.deduplicated }
              : i,
          ),
        );
        // 升级和扩容都是服务端顺带判出来的,平时这个字段根本不下发。批量上传
        // 时也只会命中一次——升上去之后后面几张就不再满足"还在 free 组"了。
        if (res.promoted) {
          const space = formatBytes(res.promoted.granted_bytes, 0);
          toast.success(
            res.promoted.group
              ? t.upload.promote.trusted(space)
              : t.upload.promote.loyalty(space),
          );
        }
        refresh();
      })
      .catch((err: Error) => {
        setItems((prev) =>
          prev.map((i) => (i.id === next.id ? { ...i, state: "error", error: err.message } : i)),
        );
      })
      .finally(() => {
        runningRef.current--;
        // Nudge the effect so the next queued item starts.
        setItems((prev) => [...prev]);
      });
  }, [items, user, refresh, t, toast]);

  // Size-weighted progress: a 20 MB photo shouldn't count the same as a 40 KB
  // icon, or the bar lurches around as small files finish first.
  const overallProgress = useMemo(() => {
    if (items.length === 0) return 0;
    // 网址取图没有大小(字节不经过浏览器),按大小加权会让它权重为零、整条进
    // 度条对它视而不见。所以这类按一张固定的"名义大小"参与加权——不准,但至
    // 少它在动。
    const weight = (i: QueueItem) => (i.size > 0 ? i.size : 512 * 1024);
    const total = items.reduce((n, i) => n + weight(i), 0);
    if (total === 0) return 0;
    const done = items.reduce((n, i) => {
      const pct = i.state === "done" ? 100 : i.state === "error" ? 0 : i.progress;
      return n + (weight(i) * pct) / 100;
    }, 0);
    return Math.round((done / total) * 100);
  }, [items]);

  const value = useMemo<UploadCtx>(
    () => ({
      items,
      enqueue,
      enqueueURL,
      remove,
      clear,
      clearFinished,
      retry,
      format,
      setFormat,
      overallProgress,
      activeCount: items.filter((i) => i.state === "queued" || i.state === "uploading").length,
      doneCount: items.filter((i) => i.state === "done").length,
      errorCount: items.filter((i) => i.state === "error").length,
    }),
    [items, enqueue, enqueueURL, remove, clear, clearFinished, retry, format, overallProgress],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useUpload(): UploadCtx {
  const v = useContext(Ctx);
  if (!v) throw new Error("useUpload must be used inside <UploadProvider>");
  return v;
}
