import { useMemo, useState, type ReactNode } from "react";

export interface HeatCell {
  /** Drives the shade. Zero or missing renders as the empty state. */
  value: number;
  tooltip: ReactNode;
}

/**
 * Shared heat-map grid. Callers supply a date→cell map and the legend text;
 * the grid, hover tooltip and four-step shading live here so the check-in and
 * upload calendars can't drift apart visually.
 *
 * Shades scale against the busiest day in range rather than an absolute
 * threshold — a quiet month should still show contrast instead of forty
 * identical dim squares.
 */
export default function HeatCalendar({
  cells,
  days = 56,
  columns = 14,
  emptyLabel = "无数据",
  summary,
}: {
  cells: Map<string, HeatCell>;
  days?: number;
  columns?: number;
  emptyLabel?: string;
  summary?: ReactNode;
}) {
  const [hover, setHover] = useState<{ date: string; cell?: HeatCell; x: number } | null>(null);

  const grid = useMemo(() => {
    const out: string[] = [];
    for (let i = days - 1; i >= 0; i--) {
      const d = new Date();
      d.setUTCDate(d.getUTCDate() - i);
      out.push(d.toISOString().slice(0, 10));
    }
    return out;
  }, [days]);

  const max = useMemo(() => Math.max(1, ...[...cells.values()].map((c) => c.value)), [cells]);

  function shade(cell?: HeatCell): string {
    if (!cell) return "heat-empty";
    if (cell.value <= 0) return "heat-zero"; // present but zero
    const ratio = cell.value / max;
    if (ratio > 0.75) return "heat-4";
    if (ratio > 0.5) return "heat-3";
    if (ratio > 0.25) return "heat-2";
    return "heat-1";
  }

  return (
    <div className="relative">
      <div
        className="grid gap-1"
        style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}
        onMouseLeave={() => setHover(null)}
      >
        {grid.map((d, i) => {
          const cell = cells.get(d);
          return (
            <div
              key={d}
              onMouseEnter={() => setHover({ date: d, cell, x: i % columns })}
              className={`aspect-square rounded-[3px] transition-colors ${shade(cell)} ${
                cell ? "cursor-help" : ""
              }`}
            />
          );
        })}
      </div>

      {hover && (
        <div
          className={`absolute -top-1 z-10 -translate-y-full rounded-lg border border-neutral-800 bg-neutral-950 px-2.5 py-1.5
                      text-[10px] whitespace-nowrap pointer-events-none shadow-lg ${
                        hover.x > columns / 2 ? "right-0" : "left-0"
                      }`}
        >
          <div className="text-neutral-300">{hover.date}</div>
          <div className="mt-0.5">{hover.cell ? hover.cell.tooltip : <span className="text-neutral-600">{emptyLabel}</span>}</div>
        </div>
      )}

      <div className="mt-2 flex items-center justify-between text-[10px] text-neutral-600">
        <span>{summary}</span>
        <span className="flex items-center gap-1">
          少
          <span className="w-2 h-2 rounded-[2px] bg-violet-700" />
          <span className="w-2 h-2 rounded-[2px] bg-violet-600" />
          <span className="w-2 h-2 rounded-[2px] bg-violet-500" />
          <span className="w-2 h-2 rounded-[2px] bg-violet-400" />
          多
        </span>
      </div>
    </div>
  );
}
