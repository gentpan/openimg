import { useMemo } from "react";
import HeatCalendar, { type HeatCell } from "./HeatCalendar";
import { formatBytes } from "../api";
import type { QuotaTransaction } from "../types";

/**
 * Upload activity, derived from the quota ledger rather than a dedicated
 * endpoint: every upload writes an `upload` row carrying a timestamp and the
 * bytes it consumed, so counting rows gives the image count and summing them
 * gives the volume — no extra table, no extra query.
 */
export default function ActivityCalendar({
  transactions,
  days = 30,
  columns = 30,
}: {
  transactions: QuotaTransaction[];
  days?: number;
  columns?: number;
}) {
  const { cells, totals } = useMemo(() => {
    const agg = new Map<string, { uploads: number; bytes: number }>();
    for (const tx of transactions) {
      if (tx.type !== "upload") continue;
      const day = tx.created_at.slice(0, 10);
      const cur = agg.get(day) ?? { uploads: 0, bytes: 0 };
      cur.uploads += 1;
      cur.bytes += Math.abs(tx.bytes); // upload rows are negative (consumption)
      agg.set(day, cur);
    }
    const cells = new Map<string, HeatCell>();
    let uploads = 0;
    let bytes = 0;
    for (const [day, v] of agg) {
      uploads += v.uploads;
      bytes += v.bytes;
      cells.set(day, {
        value: v.bytes,
        tooltip: (
          <>
            <span className="text-violet-300">上传 {v.uploads} 张</span>
            <span className="text-neutral-500 ml-1.5">占用 {formatBytes(v.bytes)}</span>
          </>
        ),
      });
    }
    return { cells, totals: { days: agg.size, uploads, bytes } };
  }, [transactions]);

  return (
    <HeatCalendar
      cells={cells}
      days={days}
      columns={columns}
      emptyLabel="没有上传"
      summary={
        <>
          {totals.days} 天有上传 · 共 <span className="text-violet-400">{totals.uploads} 张</span>
          <span className="text-neutral-600"> · {formatBytes(totals.bytes)}</span>
        </>
      }
    />
  );
}
