import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../AuthContext";
import { useUpload } from "../UploadContext";

/**
 * The drop zone. Queue state and progress live in UploadPanel (bottom-right),
 * so this stays a single-purpose target that doesn't grow as files pile up.
 */
export default function Uploader({ compact = false }: { compact?: boolean }) {
  const { user } = useAuth();
  const { enqueue } = useUpload();
  const [dragging, setDragging] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  // Paste-to-upload. Screenshot → Ctrl+V is the single most-used path on an
  // image host, so it's bound globally rather than to a focused input.
  useEffect(() => {
    if (!user) return;
    function onPaste(e: ClipboardEvent) {
      const files = Array.from(e.clipboardData?.items || [])
        .filter((i) => i.kind === "file")
        .map((i) => i.getAsFile())
        .filter((f): f is File => !!f);
      if (files.length) {
        e.preventDefault();
        enqueue(files);
      }
    }
    window.addEventListener("paste", onPaste);
    return () => window.removeEventListener("paste", onPaste);
  }, [enqueue, user]);

  return (
    <div
      onDragOver={(e) => {
        e.preventDefault();
        setDragging(true);
      }}
      onDragLeave={() => setDragging(false)}
      onDrop={(e) => {
        e.preventDefault();
        setDragging(false);
        if (user) enqueue(e.dataTransfer.files);
      }}
      onClick={() => user && inputRef.current?.click()}
      className={`relative rounded-2xl border-2 border-dashed transition cursor-pointer ${
        dragging
          ? "border-brand-500 bg-brand-950/20"
          : "border-neutral-800 bg-neutral-900/40 hover:border-neutral-700 hover:bg-neutral-900/60"
      } ${compact ? "py-8 sm:py-10" : "py-10 sm:py-16"} px-6 text-center`}
    >
      <input
        ref={inputRef}
        type="file"
        accept="image/*"
        multiple
        className="hidden"
        onChange={(e) => {
          if (e.target.files) enqueue(e.target.files);
          e.target.value = "";
        }}
      />
      <div
        className={`mx-auto mb-4 flex items-center justify-center rounded-2xl transition ${
          dragging ? "bg-brand-900/40" : "bg-neutral-900"
        } ${compact ? "w-12 h-12" : "w-16 h-16"}`}
      >
        <i className={`fa-solid fa-cloud-arrow-up text-brand-500 ${compact ? "text-xl" : "text-2xl"}`} />
      </div>

      {user ? (
        <>
          <div className={`text-neutral-200 ${compact ? "text-sm" : "text-base"}`}>
            {dragging ? "松开即可上传" : "拖拽图片到这里，或点击选择"}
          </div>
          <div className="text-xs text-neutral-500 mt-1.5">
            支持 Ctrl+V 粘贴截图 · JPEG / PNG / WebP / GIF / AVIF
          </div>
        </>
      ) : (
        <>
          <div className={`text-neutral-200 ${compact ? "text-sm" : "text-base"}`}>登录后即可上传图片</div>
          <div className="text-xs text-neutral-500 mt-1.5">注册送 1 GB，每天签到还能随机领 1–20 MB</div>
          <div className="mt-4 flex items-center justify-center gap-2">
            <Link
              to="/register"
              onClick={(e) => e.stopPropagation()}
              className="rounded-xl bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-500 transition"
            >
              免费注册
            </Link>
            <Link
              to="/login"
              onClick={(e) => e.stopPropagation()}
              className="rounded-xl bg-neutral-800 px-4 py-2 text-sm text-neutral-200 hover:bg-neutral-700 transition"
            >
              登录
            </Link>
          </div>
        </>
      )}
    </div>
  );
}
