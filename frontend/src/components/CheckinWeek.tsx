import { formatBytes } from "../api";
import type { CheckinRecord } from "../types";

/**
 * Mon–Sun as filled circles, mirroring the macOS client.
 *
 * A week is the unit people think in, and it is the smaller of the two
 * milestones — so it is the one worth showing day by day. The month gets a bar
 * instead, because thirty circles is a grid rather than a glance.
 */
export default function CheckinWeek({ records }: { records: CheckinRecord[] }) {
  const claimed = new Set(records.map((r) => r.date));
  const today = utcKey(new Date());
  const days = weekDays();

  return (
    <div className="flex">
      {days.map((d, i) => {
        const done = claimed.has(d);
        const isToday = d === today;
        return (
          <div key={d} className="flex-1 flex flex-col items-center gap-1.5">
            <div
              className={`w-8 h-8 rounded-full flex items-center justify-center text-[11px] transition ${
                done
                  ? "bg-brand-600 text-brand-ink"
                  : isToday
                    // Outlined, not filled: today is the one still available,
                    // and a hollow ring reads as "your turn" where a grey disc
                    // reads as "missed".
                    ? "border-2 border-brand-600 text-brand-300"
                    : "bg-neutral-800 text-transparent"
              } ${d > today ? "opacity-45" : ""}`}
            >
              {done && <i className="fa-solid fa-check" />}
            </div>
            <span className={`text-[10px] ${isToday ? "text-brand-300" : "text-neutral-500"}`}>
              {"一二三四五六日"[i]}
            </span>
          </div>
        );
      })}
    </div>
  );
}

export function MilestoneBar({
  title,
  current,
  total,
  reward,
}: {
  title: string;
  current: number;
  total: number;
  reward: number;
}) {
  const pct = total > 0 ? Math.min(100, (current / total) * 100) : 0;
  return (
    <div className="space-y-1.5">
      <div className="flex items-baseline justify-between">
        <span className="text-xs text-neutral-500">{title}</span>
        {reward > 0 && (
          // The number is the point. "Keep your streak" means nothing without
          // saying what the streak is worth.
          <span className="text-xs text-brand-300">
            满 {total} 天 +{formatBytes(reward, 0)}
          </span>
        )}
      </div>
      <div className="h-1.5 rounded-full bg-neutral-800 overflow-hidden">
        <div className="h-full rounded-full bg-brand-600 transition-all duration-500"
             style={{ width: `${pct}%` }} />
      </div>
      <div className="text-[10px] text-neutral-600">{current} / {total} 天</div>
    </div>
  );
}

/** The server keys days in UTC, so the strip has to be built in UTC or it
 *  drifts a day for anyone east of Greenwich — which is most users. */
function utcKey(d: Date) {
  return d.toISOString().slice(0, 10);
}

/** Monday-first, matching how the streak reads to a user. */
function weekDays(): string[] {
  const now = new Date();
  const dow = (now.getUTCDay() + 6) % 7; // 0 = Monday
  const monday = new Date(now);
  monday.setUTCDate(now.getUTCDate() - dow);
  return Array.from({ length: 7 }, (_, i) => {
    const d = new Date(monday);
    d.setUTCDate(monday.getUTCDate() + i);
    return utcKey(d);
  });
}
