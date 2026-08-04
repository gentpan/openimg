import { useMemo } from "react";
import HeatCalendar, { type HeatCell } from "./HeatCalendar";
import { formatBytes } from "../api";
import type { CheckinRecord } from "../types";

/**
 * Check-in history. The shade reflects how much space that day paid out —
 * the grant is random (1–30 MB), so a flat "checked in / didn't" square would
 * throw away the only interesting part of the data.
 */
export default function CheckinCalendar({
  records,
  days = 56,
  columns = 14,
}: {
  records: CheckinRecord[];
  days?: number;
  columns?: number;
}) {
  const { cells, total } = useMemo(() => {
    const cells = new Map<string, HeatCell>();
    let total = 0;
    for (const r of records) {
      total += r.bytes;
      cells.set(r.date, {
        value: r.bytes,
        tooltip: (
          <>
            <span className="text-violet-300">
              {r.bytes > 0 ? `+${formatBytes(r.bytes)}` : "已达上限，未发放"}
            </span>
            <span className="text-neutral-600 ml-1.5">连续 {r.streak} 天</span>
          </>
        ),
      });
    }
    return { cells, total };
  }, [records]);

  return (
    <HeatCalendar
      cells={cells}
      days={days}
      columns={columns}
      emptyLabel="未签到"
      summary={
        <>
          {records.length} 天签到 · 累计 <span className="text-violet-400">+{formatBytes(total)}</span>
        </>
      }
    />
  );
}
