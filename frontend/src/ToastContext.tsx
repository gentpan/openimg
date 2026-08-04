import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from "react";

export type ToastKind = "success" | "error" | "info";

interface Toast {
  id: number;
  kind: ToastKind;
  text: string;
  detail?: string;
  /** ms; 0 keeps it until dismissed. */
  duration: number;
}

interface ToastCtx {
  show: (t: { kind?: ToastKind; text: string; detail?: string; duration?: number }) => void;
  success: (text: string, detail?: string) => void;
  error: (text: string, detail?: string) => void;
  info: (text: string, detail?: string) => void;
  dismiss: (id: number) => void;
}

const Ctx = createContext<ToastCtx | null>(null);

const MAX_VISIBLE = 3;

/** Errors stay longer: they usually carry something the user has to read. */
const DEFAULT_MS: Record<ToastKind, number> = { success: 3500, error: 6000, info: 4500 };

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const nextId = useRef(1);
  const timers = useRef(new Map<number, number>());

  const dismiss = useCallback((id: number) => {
    setToasts((list) => list.filter((t) => t.id !== id));
    const h = timers.current.get(id);
    if (h) {
      clearTimeout(h);
      timers.current.delete(id);
    }
  }, []);

  const show = useCallback<ToastCtx["show"]>(
    ({ kind = "info", text, detail, duration }) => {
      const id = nextId.current++;
      const ms = duration ?? DEFAULT_MS[kind];
      // Oldest first out, so a burst of results doesn't push the newest one
      // off the screen before it has been read.
      setToasts((list) => [...list, { id, kind, text, detail, duration: ms }].slice(-MAX_VISIBLE));
      if (ms > 0) {
        timers.current.set(id, window.setTimeout(() => dismiss(id), ms));
      }
    },
    [dismiss],
  );

  const api: ToastCtx = {
    show,
    success: (text, detail) => show({ kind: "success", text, detail }),
    error: (text, detail) => show({ kind: "error", text, detail }),
    info: (text, detail) => show({ kind: "info", text, detail }),
    dismiss,
  };

  return (
    <Ctx.Provider value={api}>
      {children}
      <ToastStack toasts={toasts} onDismiss={dismiss} />
    </Ctx.Provider>
  );
}

export function useToast() {
  const c = useContext(Ctx);
  if (!c) throw new Error("useToast 必须在 ToastProvider 内使用");
  return c;
}

const STYLES: Record<ToastKind, { icon: string; chip: string }> = {
  success: { icon: "fa-circle-check", chip: "bg-emerald-900/40 text-emerald-300" },
  error: { icon: "fa-circle-exclamation", chip: "bg-red-900/40 text-red-300" },
  info: { icon: "fa-circle-info", chip: "bg-violet-900/40 text-violet-300" },
};

/**
 * Bottom-centre, not bottom-right: that corner already belongs to the upload
 * panel, and a transient toast stacking on top of a persistent panel means one
 * of them is always covering the other.
 *
 * pointer-events are off on the container so the strip never blocks clicks on
 * the page behind it, and back on for each toast so they stay dismissable.
 */
function ToastStack({ toasts, onDismiss }: { toasts: Toast[]; onDismiss: (id: number) => void }) {
  if (toasts.length === 0) return null;
  return (
    <div className="pointer-events-none fixed inset-x-0 bottom-4 z-[60] flex flex-col items-center gap-2 px-4">
      {toasts.map((t) => {
        const st = STYLES[t.kind];
        return (
          <button
            key={t.id}
            onClick={() => onDismiss(t.id)}
            title="点击关闭"
            className="toast-in pointer-events-auto flex w-full max-w-sm items-start gap-2.5 rounded-xl border border-neutral-800 bg-neutral-900 px-3.5 py-2.5 text-left shadow-panel"
          >
            <span
              className={`mt-px shrink-0 flex h-5 w-5 items-center justify-center rounded-md text-[10px] ${st.chip}`}
            >
              <i className={`fa-solid ${st.icon}`} />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block text-xs text-neutral-100">{t.text}</span>
              {t.detail && (
                <span className="mt-0.5 block text-[10px] leading-relaxed text-neutral-500">{t.detail}</span>
              )}
            </span>
          </button>
        );
      })}
    </div>
  );
}
