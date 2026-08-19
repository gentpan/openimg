import type { ChartOptions } from "chart.js";

/**
 * Chart colours.
 *
 * Chart.js takes concrete colour strings, not CSS variables — it paints to a
 * canvas, where `var(--x)` means nothing. So unlike the rest of the app, which
 * flips by swapping variables, charts get their palette handed to them at
 * render time.
 *
 * Still a hook rather than module-level constants: the brand hue is switchable,
 * and constants would be evaluated once at import and never change.
 */
export interface ChartTheme {
  BRAND: string;
  BRAND_DIM: string;
  SERIES: string[];
  GRID: string;
  TICK: string;
  legendLabels: { color: string; boxWidth: number; boxHeight: number; font: { size: number } };
  axisX: { grid: { color: string }; ticks: { color: string; font: { size: number } } };
  axisY: { grid: { color: string }; ticks: { color: string; font: { size: number } }; beginAtZero: boolean };
  lineBase: ChartOptions<"line">;
  barBase: ChartOptions<"bar">;
  /** Storage-composition wedges, in the order the dashboard lists them. */
  COMPOSITION: string[];
  /** Per-format bars; cycled, so order only affects which format gets which hue. */
  FORMATS: string[];
  /** The "space you didn't spend" half of the compression gauge. Green means
   *  saved, which is the one colour association worth relying on here. */
  SAVED: string;
}

const DARK = {
  BRAND: "#90ff3a",
  BRAND_DIM: "rgba(93, 227, 29, 0.15)",
  SERIES: ["#90ff3a", "#38bdf8", "#fbbf24", "#f472b6", "#94a3b8", "#2dd4bf", "#fb923c"],
  GRID: "rgba(64, 64, 64, 0.35)",
  TICK: "#737373",
  COMPOSITION: ["#90ff3a", "#38bdf8", "#fbbf24"],
  FORMATS: ["#90ff3a", "#38bdf8", "#fbbf24", "#f472b6", "#94a3b8", "#2dd4bf"],
  SAVED: "#34d399",
};


export function useChartTheme(): ChartTheme {
  const p = DARK;

  const legendLabels = { color: p.TICK, boxWidth: 10, boxHeight: 10, font: { size: 10 } };
  const axisX = { grid: { color: p.GRID }, ticks: { color: p.TICK, font: { size: 10 } } };
  const axisY = { ...axisX, beginAtZero: true };

  // Two separately-typed bases rather than one shared object: chart.js options
  // are invariant in the chart type, so a ChartOptions<"line" | "bar"> cannot
  // be spread into either one.
  const lineBase: ChartOptions<"line"> = {
    maintainAspectRatio: false,
    interaction: { mode: "index", intersect: false },
    plugins: { legend: { labels: legendLabels } },
    scales: { x: axisX, y: axisY },
  };
  const barBase: ChartOptions<"bar"> = {
    maintainAspectRatio: false,
    interaction: { mode: "index", intersect: false },
    plugins: { legend: { labels: legendLabels } },
    scales: { x: axisX, y: axisY },
  };

  return { ...p, legendLabels, axisX, axisY, lineBase, barBase };
}
