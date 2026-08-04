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
import { imageApi } from "./api";
import type { Image } from "./types";

export type ItemState = "queued" | "uploading" | "done" | "error";

export interface QueueItem {
  id: string;
  file: File;
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

interface UploadCtx {
  items: QueueItem[];
  enqueue: (files: FileList | File[]) => void;
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
  const [items, setItems] = useState<QueueItem[]>([]);
  const [format, setFormat] = useState<LinkFormat>("url");
  const runningRef = useRef(0);

  const enqueue = useCallback((files: FileList | File[]) => {
    const picked = Array.from(files).filter((f) => f.type.startsWith("image/"));
    if (picked.length === 0) return;
    setItems((prev) => [
      ...prev,
      ...picked.map((file) => ({
        id: `${file.name}-${file.size}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
        file,
        preview: URL.createObjectURL(file),
        state: "queued" as ItemState,
        progress: 0,
      })),
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

    imageApi
      .upload(next.file, {
        onProgress: (pct) =>
          setItems((prev) => prev.map((i) => (i.id === next.id ? { ...i, progress: pct } : i))),
      })
      .then((res) => {
        setItems((prev) =>
          prev.map((i) =>
            i.id === next.id
              ? { ...i, state: "done", progress: 100, image: res.image, deduplicated: res.deduplicated }
              : i,
          ),
        );
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
  }, [items, user, refresh]);

  // Size-weighted progress: a 20 MB photo shouldn't count the same as a 40 KB
  // icon, or the bar lurches around as small files finish first.
  const overallProgress = useMemo(() => {
    if (items.length === 0) return 0;
    const total = items.reduce((n, i) => n + i.file.size, 0);
    if (total === 0) return 0;
    const done = items.reduce((n, i) => {
      const pct = i.state === "done" ? 100 : i.state === "error" ? 0 : i.progress;
      return n + (i.file.size * pct) / 100;
    }, 0);
    return Math.round((done / total) * 100);
  }, [items]);

  const value = useMemo<UploadCtx>(
    () => ({
      items,
      enqueue,
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
    [items, enqueue, remove, clear, clearFinished, retry, format, overallProgress],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useUpload(): UploadCtx {
  const v = useContext(Ctx);
  if (!v) throw new Error("useUpload must be used inside <UploadProvider>");
  return v;
}
