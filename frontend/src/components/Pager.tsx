import { useLang } from "../LangContext";

/**
 * Page navigation for the ledger.
 *
 * Shows a window of page numbers around the current one rather than all of
 * them — a ledger with 40 pages would otherwise render 40 buttons and wrap
 * across three lines.
 */
export default function Pager({
  page,
  perPage,
  total,
  busy,
  onPage,
}: {
  page: number;
  perPage: number;
  total: number;
  busy: boolean;
  onPage: (p: number) => void;
}) {
  const { t } = useLang();
  const pages = Math.ceil(total / perPage);
  const from = page * perPage + 1;
  const to = Math.min(total, (page + 1) * perPage);

  // A five-wide window, clamped so it never runs past either end.
  const start = Math.max(0, Math.min(page - 2, pages - 5));
  const window = Array.from({ length: Math.min(5, pages) }, (_, i) => start + i);

  const btn = "min-w-7 h-7 px-1.5 rounded-md text-[11px] transition disabled:opacity-40";

  return (
    <div className="flex flex-wrap items-center gap-1.5 px-5 py-3 border-t border-neutral-800/60">
      <span className="text-[10px] text-neutral-600">
        {from}–{to} / {total}
      </span>
      <div className="flex-1" />

      <button
        onClick={() => onPage(page - 1)}
        disabled={page === 0 || busy}
        className={`${btn} text-neutral-400 hover:bg-neutral-800 hover:text-neutral-100`}
        title={t.common.pager.prev}
      >
        <i className="fa-solid fa-chevron-left text-[9px]" />
      </button>

      {start > 0 && (
        <>
          <button onClick={() => onPage(0)} disabled={busy} className={`${btn} text-neutral-400 hover:bg-neutral-800`}>
            1
          </button>
          <span className="text-[10px] text-faint">…</span>
        </>
      )}

      {window.map((p) => (
        <button
          key={p}
          onClick={() => onPage(p)}
          disabled={busy}
          className={`${btn} tabular-nums ${
            p === page
              ? "bg-brand-600 text-brand-ink"
              : "text-neutral-400 hover:bg-neutral-800 hover:text-neutral-100"
          }`}
        >
          {p + 1}
        </button>
      ))}

      {start + 5 < pages && (
        <>
          <span className="text-[10px] text-faint">…</span>
          <button
            onClick={() => onPage(pages - 1)}
            disabled={busy}
            className={`${btn} text-neutral-400 hover:bg-neutral-800`}
          >
            {pages}
          </button>
        </>
      )}

      <button
        onClick={() => onPage(page + 1)}
        disabled={page >= pages - 1 || busy}
        className={`${btn} text-neutral-400 hover:bg-neutral-800 hover:text-neutral-100`}
        title={t.common.pager.next}
      >
        <i className="fa-solid fa-chevron-right text-[9px]" />
      </button>
    </div>
  );
}
