import Icon from "../Icon";
import { useEffect, useRef, useState } from "react";
import { useLang } from "../LangContext";

/** The sizes offered. Shared so the gallery and the admin ledger agree. */
export const PAGE_SIZES = [25, 50, 100, 200];

/** Sort picker. A popover rather than a native <select> so it can carry icons
 *  and match the rest of the dark chrome. */
export default function PageSizeMenu({
  value,
  onChange,
}: {
  value: number;
  onChange: (v: number) => void;
}) {
  const { t } = useLang();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function onDoc(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  return (
    <div className="relative" ref={ref}>
      <button
        onClick={() => setOpen((o) => !o)}
        title={t.gallery.pageSizeTitle}
        className="flex items-center gap-1.5 rounded-lg bg-neutral-900 border border-neutral-800 px-3 py-1.5 text-xs text-neutral-300 hover:border-neutral-700 transition"
      >
        <Icon name="table-cells" className="text-[10px] text-neutral-500"  />
        <span className="tabular-nums">{value}</span>
        <Icon name="chevron-down" className="text-[8px] text-neutral-600"  />
      </button>

      {open && (
        <div className="absolute right-0 top-full mt-1.5 z-20 w-28 rounded-xl border border-neutral-800 bg-neutral-900 py-1 shadow-panel">
          {PAGE_SIZES.map((n) => (
            <button
              key={n}
              onClick={() => {
                onChange(n);
                setOpen(false);
              }}
              className={`flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs transition ${
                n === value ? "text-brand-300 bg-brand-950/30" : "text-neutral-400 hover:bg-neutral-800/60"
              }`}
            >
              <span className="tabular-nums">{n}</span>
              <span className="text-neutral-600">{t.gallery.pageSizeUnit}</span>
              {n === value && <Icon name="check" className="ml-auto text-[9px]"  />}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
