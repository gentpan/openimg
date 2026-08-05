import { useEffect, useRef, useState } from "react";

/** Respects the OS reduced-motion setting — animations are decoration, never information. */
function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    const mq = window.matchMedia?.("(prefers-reduced-motion: reduce)");
    if (!mq) return;
    setReduced(mq.matches);
    const on = (e: MediaQueryListEvent) => setReduced(e.matches);
    mq.addEventListener?.("change", on);
    return () => mq.removeEventListener?.("change", on);
  }, []);
  return reduced;
}

/**
 * Counts from zero to `value` on mount. Uses an ease-out curve so the number
 * lands softly instead of stopping dead.
 */
export function useCountUp(value: number, duration = 900): number {
  const reduced = usePrefersReducedMotion();
  const [n, setN] = useState(reduced ? value : 0);
  const frame = useRef(0);
  const startedAt = useRef(0);
  const from = useRef(0);

  useEffect(() => {
    if (reduced) {
      setN(value);
      return;
    }
    from.current = n;
    startedAt.current = 0;

    const tick = (ts: number) => {
      if (!startedAt.current) startedAt.current = ts;
      const t = Math.min(1, (ts - startedAt.current) / duration);
      const eased = 1 - Math.pow(1 - t, 3);
      setN(from.current + (value - from.current) * eased);
      if (t < 1) frame.current = requestAnimationFrame(tick);
    };
    frame.current = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame.current);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, duration, reduced]);

  return n;
}

/**
 * Segmented bar meter — the filled segments light up left to right on mount.
 *
 * Chosen over a plain progress bar because discrete segments make a small
 * percentage legible: 2% of a solid bar is an invisible sliver, but it's
 * clearly "one bar out of forty" here.
 */
export function BarMeter({
  percent,
  bars = 40,
  className = "",
  tone = "accent",
}: {
  percent: number;
  bars?: number;
  className?: string;
  tone?: "accent" | "amber";
}) {
  const reduced = usePrefersReducedMotion();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    // A frame's delay lets the browser paint the empty state first, so the
    // fill actually animates instead of appearing already done.
    const t = setTimeout(() => setMounted(true), 40);
    return () => clearTimeout(t);
  }, []);

  const clamped = Math.max(0, Math.min(100, percent));

  // Fill to the exact fraction, letting the last lit segment be partial.
  //
  // Rounding to whole segments quantises the bar to 100/bars — at 50 segments
  // that is 2% a step, so 3.3% rounds up and draws 4% next to a label reading
  // 3.3%. The number and the picture then disagree about the same fact, and
  // the picture is the one people trust.
  const exact = (clamped / 100) * bars;
  const whole = Math.floor(exact);
  // A sliver rather than nothing, so "barely used" never reads as "unused".
  const partial = clamped <= 0 ? 0 : Math.max(exact - whole, whole === 0 ? 0.35 : 0);
  const animated = reduced || mounted;

  const on = tone === "amber" ? "bg-amber-400" : "bg-accent-400";
  // 15% of a brand colour over a white card is white. The unlit cells are
  // structure, not accent, so they use the neutral track instead.
  const off = "bg-neutral-800";

  return (
    <div className={`flex items-stretch gap-[2px] ${className}`} aria-hidden="true">
      {Array.from({ length: bars }, (_, i) => {
        // Full, partly full, or empty.
        const fill = !animated ? 0 : i < whole ? 1 : i === whole ? partial : 0;
        return (
          <div key={i} className={`relative flex-1 overflow-hidden rounded-full ${off}`}>
            <span
              className={`absolute inset-y-0 left-0 rounded-full ${on} transition-[width] duration-300`}
              style={{
                width: `${fill * 100}%`,
                transitionDelay: reduced ? "0ms" : `${i * 14}ms`,
              }}
            />
          </div>
        );
      })}
    </div>
  );
}

export interface AreaPoint {
  label: string;
  value: number;
}

/**
 * Filled sparkline. Hand-rolled SVG rather than chart.js: this is a decorative
 * trend, and a stroke-dash draw-on animation is far simpler here than fighting
 * a charting library's animation config.
 */
export function AreaSpark({
  data,
  height = 120,
  formatValue = (n: number) => String(n),
  className = "",
}: {
  data: AreaPoint[];
  height?: number;
  formatValue?: (n: number) => string;
  className?: string;
}) {
  const reduced = usePrefersReducedMotion();
  const [drawn, setDrawn] = useState(false);
  const [hover, setHover] = useState<number | null>(null);
  const id = useRef(`spark-${Math.random().toString(36).slice(2, 9)}`).current;

  useEffect(() => {
    const t = setTimeout(() => setDrawn(true), 60);
    return () => clearTimeout(t);
  }, []);

  if (data.length === 0) {
    return (
      <div className={`flex items-center justify-center text-xs text-neutral-600 ${className}`} style={{ height }}>
        暂无数据
      </div>
    );
  }

  const W = 1000;
  const H = 260;
  const pad = 8;
  const max = Math.max(...data.map((d) => d.value), 1);
  const stepX = data.length > 1 ? (W - pad * 2) / (data.length - 1) : 0;

  const pts = data.map((d, i) => ({
    x: pad + i * stepX,
    y: H - pad - (d.value / max) * (H - pad * 2),
  }));

  // Catmull-Rom style smoothing — straight segments look jagged on noisy
  // daily data, and a fully smooth spline overshoots below zero.
  const line = pts
    .map((p, i) => {
      if (i === 0) return `M ${p.x} ${p.y}`;
      const prev = pts[i - 1];
      const cx = (prev.x + p.x) / 2;
      return `C ${cx} ${prev.y}, ${cx} ${p.y}, ${p.x} ${p.y}`;
    })
    .join(" ");
  const area = `${line} L ${pts[pts.length - 1].x} ${H} L ${pts[0].x} ${H} Z`;

  const active = hover != null ? data[hover] : null;

  return (
    <div className={`relative ${className}`} style={{ height }}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        preserveAspectRatio="none"
        className="w-full h-full overflow-visible"
        onMouseLeave={() => setHover(null)}
        onMouseMove={(e) => {
          const rect = e.currentTarget.getBoundingClientRect();
          const rel = ((e.clientX - rect.left) / rect.width) * W;
          const idx = Math.round((rel - pad) / (stepX || 1));
          setHover(Math.max(0, Math.min(data.length - 1, idx)));
        }}
      >
        <defs>
          <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--color-accent-600)" stopOpacity="0.45" />
            <stop offset="100%" stopColor="var(--color-accent-600)" stopOpacity="0" />
          </linearGradient>
        </defs>

        <path
          d={area}
          fill={`url(#${id})`}
          style={{
            opacity: reduced || drawn ? 1 : 0,
            transition: reduced ? undefined : "opacity 700ms ease-out 250ms",
          }}
        />
        <path
          d={line}
          fill="none"
          stroke="var(--color-accent-400)"
          strokeWidth={2.5}
          vectorEffect="non-scaling-stroke"
          strokeLinecap="round"
          style={
            reduced
              ? undefined
              : {
                  strokeDasharray: 2200,
                  strokeDashoffset: drawn ? 0 : 2200,
                  transition: "stroke-dashoffset 1100ms cubic-bezier(0.4, 0, 0.2, 1)",
                }
          }
        />

        {hover != null && (
          <>
            <line
              x1={pts[hover].x}
              y1={0}
              x2={pts[hover].x}
              y2={H}
              stroke="var(--color-accent-400)"
              strokeWidth={1}
              strokeDasharray="3 3"
              vectorEffect="non-scaling-stroke"
              opacity={0.5}
            />
            <circle cx={pts[hover].x} cy={pts[hover].y} r={4} fill="var(--color-accent-400)" vectorEffect="non-scaling-stroke" />
          </>
        )}
      </svg>

      {active && (
        <div className="absolute top-0 left-0 right-0 flex justify-center pointer-events-none">
          <div className="rounded-lg bg-neutral-950/90 border border-neutral-800 px-2 py-1 text-[10px] text-neutral-300 whitespace-nowrap">
            {active.label} · <span className="text-accent-300">{formatValue(active.value)}</span>
          </div>
        </div>
      )}
    </div>
  );
}
