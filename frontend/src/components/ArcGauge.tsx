import { useEffect, useState } from "react";
import { useLang } from "../LangContext";

export interface Segment {
  label: string;
  value: number;
  color: string;
}

/**
 * Semicircular segmented gauge with a legend beside it.
 *
 * A half-arc rather than a full ring because the composition it shows is a
 * single total split N ways — the flat bottom edge reads as a baseline, and it
 * leaves room for the legend without shrinking the chart.
 *
 * Segments sweep in on mount via stroke-dashoffset, and each carries a small
 * gap so adjacent colours never bleed into each other.
 */
export default function ArcGauge({
  segments,
  size = 160,
  thickness = 18,
  formatValue,
  className = "",
}: {
  segments: Segment[];
  size?: number;
  thickness?: number;
  formatValue?: (v: number) => string;
  className?: string;
}) {
  const [drawn, setDrawn] = useState(false);
  useEffect(() => {
    const t = setTimeout(() => setDrawn(true), 60);
    return () => clearTimeout(t);
  }, []);

  const total = segments.reduce((n, s) => n + s.value, 0);
  const live = segments.filter((s) => s.value > 0);

  const r = (size - thickness) / 2;
  const cy = size / 2;
  const arcLen = Math.PI * r; // half circumference
  const GAP = live.length > 1 ? 4 : 0;

  let offset = 0;
  const drawn_segments = live.map((s) => {
    const len = total > 0 ? (s.value / total) * arcLen : 0;
    const seg = { ...s, len: Math.max(0, len - GAP), start: offset };
    offset += len;
    return seg;
  });

  return (
    <div className={`flex items-center gap-4 ${className}`}>
      <div className="relative shrink-0" style={{ width: size, height: size / 2 + thickness / 2 }}>
        <svg width={size} height={size / 2 + thickness / 2} viewBox={`0 0 ${size} ${size / 2 + thickness / 2}`}>
          {/* Track */}
          <path
            d={`M ${thickness / 2} ${cy} A ${r} ${r} 0 0 1 ${size - thickness / 2} ${cy}`}
            fill="none"
            stroke="var(--track)"
            strokeWidth={thickness}
            strokeLinecap="round"
          />
          {drawn_segments.map((s, i) => (
            <path
              key={i}
              d={`M ${thickness / 2} ${cy} A ${r} ${r} 0 0 1 ${size - thickness / 2} ${cy}`}
              fill="none"
              stroke={s.color}
              strokeWidth={thickness}
              strokeLinecap="round"
              strokeDasharray={`${s.len} ${arcLen}`}
              style={{
                strokeDashoffset: drawn ? -s.start : -arcLen,
                transition: `stroke-dashoffset 900ms cubic-bezier(0.4,0,0.2,1) ${i * 90}ms`,
              }}
            />
          ))}
        </svg>
      </div>

      <div className="flex-1 min-w-0 space-y-1.5">
        {segments.map((s) => (
          <div
            key={s.label}
            className="flex items-center gap-2 rounded-lg bg-neutral-950/60 px-2.5 py-1.5"
          >
            <span className="h-3 w-[3px] shrink-0 rounded-full" style={{ backgroundColor: s.color }} />
            <span className="flex-1 min-w-0 truncate text-[11px] text-neutral-400">{s.label}</span>
            <span className="shrink-0 text-xs font-brand text-neutral-100 tabular-nums">
              {formatValue ? formatValue(s.value) : `${total > 0 ? Math.round((s.value / total) * 100) : 0}%`}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

export interface CategoryRow {
  label: string;
  value: number;
  note: string;
  color: string;
}

/**
 * Ranked bars, one colour per row. Each bar is scaled against the largest row
 * rather than the total — with a long tail, scaling to the total leaves every
 * bar but the first as an invisible stub.
 */
export function CategoryBars({ rows, className = "" }: { rows: CategoryRow[]; className?: string }) {
  const { t } = useLang();
  const [drawn, setDrawn] = useState(false);
  useEffect(() => {
    const t = setTimeout(() => setDrawn(true), 60);
    return () => clearTimeout(t);
  }, []);

  const max = Math.max(1, ...rows.map((r) => r.value));

  if (rows.length === 0) {
    return <div className={`py-8 text-center text-xs text-neutral-600 ${className}`}>{t.common.noData}</div>;
  }

  return (
    <div className={`space-y-3 ${className}`}>
      {rows.map((r, i) => (
        <div key={r.label}>
          <div className="flex items-baseline justify-between mb-1.5">
            <span className="text-xs text-neutral-300">{r.label}</span>
            <span className="text-[11px] text-neutral-600">{r.note}</span>
          </div>
          <div className="h-1.5 rounded-full bg-neutral-800 overflow-hidden">
            <div
              className="h-full rounded-full"
              style={{
                width: drawn ? `${(r.value / max) * 100}%` : "0%",
                backgroundColor: r.color,
                transition: `width 800ms cubic-bezier(0.4,0,0.2,1) ${i * 80}ms`,
              }}
            />
          </div>
        </div>
      ))}
    </div>
  );
}
