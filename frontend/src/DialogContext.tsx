import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useLang } from "./LangContext";

/**
 * Styled replacements for window.alert / confirm / prompt.
 *
 * Promise-based so call sites keep their shape: `if (!confirm(x)) return`
 * becomes `if (!(await dialog.confirm({ title: x }))) return` — the decision
 * still reads top-to-bottom instead of being scattered into open/close
 * handlers and callback props.
 *
 * The native dialogs were not just ugly: they block the whole tab, ignore the
 * app's theme entirely, and confirm() only offers OK/Cancel — which is why the
 * admin ban flow had to chain two confirms to express three outcomes. choose()
 * exists for exactly that case.
 */

type ButtonKind = "primary" | "danger" | "ghost";

interface DialogButton {
  label: string;
  value: string;
  kind: ButtonKind;
}

interface Pending {
  /** Remount key: consecutive dialogs may share a title, and the input state
   *  must not leak from one into the next. */
  seq: number;
  title: string;
  body?: ReactNode;
  input?: { initial: string; placeholder?: string };
  buttons: DialogButton[];
  /** null = dismissed (ESC, backdrop, cancel). */
  resolve: (r: { value: string; text: string } | null) => void;
}

interface DialogApi {
  alert(opts: { title: string; body?: ReactNode }): Promise<void>;
  confirm(opts: {
    title: string;
    body?: ReactNode;
    danger?: boolean;
    confirmLabel?: string;
  }): Promise<boolean>;
  prompt(opts: {
    title: string;
    body?: ReactNode;
    initial?: string;
    placeholder?: string;
  }): Promise<string | null>;
  choose(opts: {
    title: string;
    body?: ReactNode;
    options: { label: string; value: string; danger?: boolean }[];
  }): Promise<string | null>;
}

const Ctx = createContext<DialogApi | null>(null);

export function DialogProvider({ children }: { children: ReactNode }) {
  const { t } = useLang();
  const [pending, setPending] = useState<Pending | null>(null);

  const seq = useRef(0);
  const open = useCallback(
    (p: Omit<Pending, "resolve" | "seq">) =>
      new Promise<{ value: string; text: string } | null>((resolve) => {
        setPending((prev) => {
          // A second dialog while one is up should never happen, but if it
          // does, the first caller must not hang forever on an orphaned
          // promise.
          prev?.resolve(null);
          seq.current += 1;
          return { ...p, seq: seq.current, resolve };
        });
      }).finally(() => setPending(null)),
    [],
  );

  const api: DialogApi = {
    alert: ({ title, body }) =>
      open({
        title,
        body,
        buttons: [{ label: t.common.ok, value: "ok", kind: "primary" }],
      }).then(() => undefined),

    confirm: ({ title, body, danger, confirmLabel }) =>
      open({
        title,
        body,
        buttons: [
          { label: t.common.cancel, value: "cancel", kind: "ghost" },
          {
            label: confirmLabel ?? t.common.ok,
            value: "ok",
            kind: danger ? "danger" : "primary",
          },
        ],
      }).then((r) => r?.value === "ok"),

    prompt: ({ title, body, initial = "", placeholder }) =>
      open({
        title,
        body,
        input: { initial, placeholder },
        buttons: [
          { label: t.common.cancel, value: "cancel", kind: "ghost" },
          { label: t.common.ok, value: "ok", kind: "primary" },
        ],
      }).then((r) => (r?.value === "ok" ? r.text : null)),

    choose: ({ title, body, options }) =>
      open({
        title,
        body,
        buttons: [
          { label: t.common.cancel, value: "cancel", kind: "ghost" },
          ...options.map((o) => ({
            label: o.label,
            value: o.value,
            kind: (o.danger ? "danger" : "primary") as ButtonKind,
          })),
        ],
        // Ghost buttons resolve null in the host, so a surviving value is
        // always one of the caller's options.
      }).then((r) => r?.value ?? null),
  };

  return (
    <Ctx.Provider value={api}>
      {children}
      {pending && <DialogHost key={pending.seq} p={pending} />}
    </Ctx.Provider>
  );
}

function DialogHost({ p }: { p: Pending }) {
  const [text, setText] = useState(p.input?.initial ?? "");
  const primaryRef = useRef<HTMLButtonElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Enter in the input submits the last button, which for prompt() is its
  // only primary action.
  const submit = () => p.resolve({ value: p.buttons[p.buttons.length - 1].value, text });

  // Autofocus never lands on a danger button: focus plus a reflexive Enter
  // must not equal "delete everything". Prefer the last non-danger action,
  // fall back to the first button (cancel).
  const focusIdx = (() => {
    for (let i = p.buttons.length - 1; i >= 0; i--) {
      if (p.buttons[i].kind !== "danger") return i;
    }
    return 0;
  })();
  const dismiss = () => p.resolve(null);

  useEffect(() => {
    (p.input ? inputRef.current : primaryRef.current)?.focus();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") dismiss();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const kindCls: Record<ButtonKind, string> = {
    primary: "bg-brand-600 text-brand-ink hover:bg-brand-500",
    danger: "bg-red-600 text-white hover:bg-red-700",
    ghost: "text-neutral-400 hover:text-neutral-100",
  };

  return (
    <div className="fixed inset-0 z-[70] flex items-center justify-center px-4">
      <div className="absolute inset-0 bg-black/50" onClick={dismiss} />
      <div
        role="dialog"
        aria-modal="true"
        className="relative w-full max-w-sm rounded-2xl border border-neutral-800 bg-neutral-900 p-5 shadow-panel"
      >
        <div className="text-sm text-neutral-100">{p.title}</div>
        {p.body && (
          <div className="mt-1.5 text-[11px] leading-relaxed text-neutral-500 whitespace-pre-line">
            {p.body}
          </div>
        )}

        {p.input && (
          <input
            ref={inputRef}
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && submit()}
            placeholder={p.input.placeholder}
            className="mt-3 w-full rounded-lg bg-neutral-950 border border-neutral-800 px-3 py-2 text-sm text-neutral-100 outline-none focus:border-neutral-600"
          />
        )}

        <div className="mt-4 flex flex-wrap items-center justify-end gap-2">
          {p.buttons.map((b, i) => {
            return (
              <button
                key={b.value}
                ref={i === focusIdx ? primaryRef : undefined}
                onClick={() =>
                  b.kind === "ghost" ? dismiss() : p.resolve({ value: b.value, text })
                }
                className={`whitespace-nowrap rounded-lg px-3 py-1.5 text-xs font-medium transition ${kindCls[b.kind]}`}
              >
                {b.label}
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}

export function useDialog() {
  const c = useContext(Ctx);
  if (!c) throw new Error("useDialog 必须在 DialogProvider 内使用");
  return c;
}
