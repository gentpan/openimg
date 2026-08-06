import { useMemo } from "react";
import HeatCalendar, { type HeatCell } from "./HeatCalendar";
import { formatBytes } from "../api";
import type { CheckinRecord } from "../types";
import { useLang } from "../LangContext";

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
  const { t } = useLang();
  const { cells, total } = useMemo(() => {
    const cells = new Map<string, HeatCell>();
    let total = 0;
    for (const r of records) {
      total += r.bytes;
      cells.set(r.date, {
        value: r.bytes,
        tooltip: (
          <>
            <span className="text-brand-300">
              {r.bytes > 0 ? `+${formatBytes(r.bytes)}` : t.checkinCalendar.cappedNoGrant}
            </span>
            <span className="text-neutral-600 ml-1.5">{t.checkinCalendar.streak(r.streak)}</span>
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
      emptyLabel={t.checkinCalendar.empty}
      summary={
        <>
            {t.checkinCalendar.summary(records.length, formatBytes(total))}
        </>
      }
    />
  );
}
