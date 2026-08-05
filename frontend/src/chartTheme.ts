import type { ChartOptions } from "chart.js";
import { useTheme } from "./ThemeContext";

/**
 * Chart colours, per theme.
 *
 * Chart.js takes concrete colour strings, not CSS variables — it paints to a
 * canvas, where `var(--x)` means nothing. So unlike the rest of the app, which
 * flips by swapping variables, charts need their palette handed to them at
 * render time. Hence a hook rather than the module-level constants these
 * replaced: those were evaluated once at import and could never change.
 *
 * The dark values are exactly what the charts used before the light theme
 * existed. The light ones are the same hues stepped down to the 600/700 tier —
 * the 300/400 tiers these started from sit at 1.5–2:1 against white, which is
 * below the 3:1 that non-text graphics need to be distinguishable.
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
  BRAND: "#5de31d",
  BRAND_DIM: "rgba(93, 227, 29, 0.15)",
  SERIES: ["#5de31d", "#38bdf8", "#fbbf24", "#f472b6", "#94a3b8", "#2dd4bf", "#fb923c"],
  GRID: "rgba(64, 64, 64, 0.35)",
  TICK: "#737373",
  COMPOSITION: ["#5de31d", "#38bdf8", "#fbbf24"],
  FORMATS: ["#5de31d", "#38bdf8", "#fbbf24", "#f472b6", "#94a3b8", "#2dd4bf"],
  SAVED: "#34d399",
};

const LIGHT = {
  BRAND: "#46b312",
  BRAND_DIM: "rgba(70, 179, 18, 0.18)",
  SERIES: ["#46b312", "#0284c7", "#d97706", "#db2777", "#475569", "#0d9488", "#ea580c"],
  GRID: "rgba(17, 24, 39, 0.12)",
  TICK: "#6b7280",
  COMPOSITION: ["#46b312", "#0284c7", "#d97706"],
  FORMATS: ["#46b312", "#0284c7", "#d97706", "#db2777", "#475569", "#0d9488"],
  SAVED: "#059669",
};

export function useChartTheme(): ChartTheme {
  const { theme } = useTheme();
  const p = theme === "light" ? LIGHT : DARK;

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
