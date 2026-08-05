import { useCallback, useEffect, useState } from "react";
import { Navigate } from "react-router-dom";
import { useAuth } from "../AuthContext";
import Footer from "../Footer";
import Nav from "../components/Nav";
import { formatBytes, quotaApi } from "../api";
import { BarMeter, useCountUp } from "../components/Meters";
import CheckinCalendar from "../components/CheckinCalendar";
import CheckinWeek, { MilestoneBar } from "../components/CheckinWeek";
import type { CheckinRecord, QuotaInfo, QuotaTransaction } from "../types";
import { RingSpinner } from "../components/Spinner";

const TX_LABELS: Record<string, { label: string; icon: string; tone: string }> = {
  signup_grant: { label: "注册赠送", icon: "fa-gift", tone: "text-brand-300" },
  checkin: { label: "每日签到", icon: "fa-calendar-check", tone: "text-brand-300" },
  referral: { label: "邀请奖励", icon: "fa-user-plus", tone: "text-brand-300" },
  admin_grant: { label: "管理员调整", icon: "fa-user-shield", tone: "text-amber-300" },
  upload: { label: "上传占用", icon: "fa-cloud-arrow-up", tone: "text-neutral-400" },
  delete_refund: { label: "删除释放", icon: "fa-trash-can", tone: "text-brand-300" },
};

/** Days into the current month-long milestone. */
function monthDone(streak: number, total: number) {
  const t = total || 30;
  if (streak <= 0) return 0;
  return streak % t === 0 ? t : streak % t;
}

export default function SpacePage() {
  const { user, loading, refresh } = useAuth();
  const [quota, setQuota] = useState<QuotaInfo | null>(null);
  const [txs, setTxs] = useState<QuotaTransaction[]>([]);
  const [history, setHistory] = useState<CheckinRecord[]>([]);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ kind: "ok" | "err"; text: string } | null>(null);

  // The ledger is paged server-side. It only grows — every upload, delete and
  // check-in appends — so fetching it whole was going to get slower forever.
  const [txTotal, setTxTotal] = useState(0);
  const [perPage, setPerPage] = useState(20);
  const [page, setPage] = useState(0);
  const [txBusy, setTxBusy] = useState(false);

  const loadTxs = useCallback(async (p: number, n: number) => {
    setTxBusy(true);
    try {
      const r = await quotaApi.transactions(n, p * n);
      setTxs(r.transactions);
      setTxTotal(r.total);
    } finally {
      setTxBusy(false);
    }
  }, []);

  async function load() {
    const [q, h] = await Promise.all([quotaApi.info(), quotaApi.checkinHistory(60)]);
    setQuota(q);
    setHistory(h);
  }

  useEffect(() => {
    if (user) load().catch(() => {});
  }, [user]);

  useEffect(() => {
    if (user) loadTxs(page, perPage).catch(() => {});
  }, [user, page, perPage, loadTxs]);

  async function doCheckin() {
    setBusy(true);
    setMsg(null);
    try {
      const res = await quotaApi.checkin();
      setMsg({
        kind: "ok",
        text: res.capped
          ? "签到成功，但空间已达上限，本次未发放"
          : `签到成功，+${formatBytes(res.granted_bytes)}，已连续 ${res.streak} 天`,
      });
      await Promise.all([load(), refresh()]);
    } catch (e) {
      setMsg({ kind: "err", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(false);
    }
  }

  const usedAnimated = useCountUp(quota?.used_bytes ?? 0, 1000);
  const usedPct = quota && quota.quota_bytes > 0 ? (quota.used_bytes / quota.quota_bytes) * 100 : 0;

  if (loading) return <Center>加载中…</Center>;
  if (!user) return <Navigate to="/login" replace />;


  return (
    <div className="min-h-screen flex flex-col bg-neutral-950">
      <Nav />
      <div className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 py-8">
        <h1 className="text-lg font-brand text-neutral-100 mb-5">我的空间</h1>

        {msg && (
          <div
            className={`mb-4 rounded-xl border px-4 py-2.5 text-xs ${
              msg.kind === "ok"
                ? "border-teal-500/30 bg-teal-950/20 text-teal-200"
                : "border-red-500/30 bg-red-950/20 text-red-200"
            }`}
          >
            {msg.text}
          </div>
        )}

        <div className="grid lg:grid-cols-3 gap-3 mb-8">
          {/* Quota meter */}
          <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-5">
            <div className="text-[10px] uppercase tracking-wider text-neutral-600 mb-3">空间使用</div>
            {quota && (
              <>
                <div className="text-3xl font-brand text-neutral-100 tabular-nums">
                  {formatBytes(usedAnimated, 1)}
                </div>
                <div className="text-xs text-neutral-600 mt-1 mb-4">
                  共 {formatBytes(quota.quota_bytes)} · 已用 {usedPct.toFixed(1)}%
                </div>
                <BarMeter percent={usedPct} bars={50} tone={usedPct >= 90 ? "amber" : "brand"} className="h-10" />
                <dl className="mt-4 space-y-1 text-[11px]">
                  <Row label="总容量" value={formatBytes(quota.quota_bytes)} />
                  <Row label="剩余可用" value={formatBytes(quota.available_bytes)} />
                  <Row label="图片数" value={`${quota.image_count} 张`} />
                </dl>
              </>
            )}
          </div>

          {/* Check-in */}
          <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-5 lg:col-span-2">
            <div className="flex items-start justify-between mb-4">
              <div>
                <div className="text-xs text-neutral-400">每日签到</div>
                <div className="mt-1.5 text-2xl font-brand text-neutral-100">
                  {quota?.checkin.streak ?? 0}
                  <span className="text-xs text-neutral-500 ml-1.5">天连续</span>
                </div>
                {quota && (
                  <div className="mt-1 text-[11px] text-neutral-600">
                    每天随机 {formatBytes(quota.checkin.min_bytes, 0)} –{" "}
                    {formatBytes(quota.checkin.max_bytes, 0)}
                    <span className="ml-1 text-faint">· 永久累加，不会过期</span>
                  </div>
                )}
              </div>
              <button
                onClick={doCheckin}
                disabled={busy || quota?.checkin.checked_in_today}
                className="rounded-xl bg-brand-600 px-5 py-2.5 text-sm font-medium text-brand-ink hover:bg-brand-600 disabled:bg-neutral-800 disabled:text-neutral-600 transition whitespace-nowrap"
              >
                {busy ? (
                  <RingSpinner className="h-3.5 w-3.5 inline-block align-[-2px]" />
                ) : quota?.checkin.checked_in_today ? (
                  "今日已签"
                ) : (
                  <>
                    签到领 {formatBytes(quota?.checkin.next_min_bytes ?? 0, 0)}–
                    {formatBytes(quota?.checkin.next_max_bytes ?? 0, 0)}
                  </>
                )}
              </button>
            </div>

            {quota && (
              <div className="mb-4 space-y-4">
                <CheckinWeek records={history} />
                <MilestoneBar
                  title="本月进度"
                  current={monthDone(quota.checkin.streak, quota.checkin.days_per_month)}
                  total={quota.checkin.days_per_month || 30}
                  reward={quota.checkin.month_bonus}
                />
              </div>
            )}

            <CheckinCalendar records={history} days={56} columns={14} />
          </div>
        </div>

        {/* Ledger */}
        <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 overflow-hidden">
          <div className="flex items-center gap-2 px-5 py-3 border-b border-neutral-800/60">
            <span className="text-xs text-neutral-300">空间流水</span>
            {txTotal > 0 && <span className="text-[10px] text-neutral-600">共 {txTotal} 条</span>}
            {txBusy && <RingSpinner className="h-3 w-3 text-brand-400" />}
            <div className="flex-1" />
            <label className="flex items-center gap-1.5 text-[10px] text-neutral-600">
              每页
              <select
                value={perPage}
                onChange={(e) => {
                  setPerPage(Number(e.target.value));
                  setPage(0);
                }}
                className="rounded-md bg-neutral-950 border border-neutral-800 px-1.5 py-1 text-[10px] text-neutral-300 outline-none focus:border-brand-500"
              >
                {[10, 20, 50, 100].map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
              条
            </label>
          </div>
          {txs.length === 0 ? (
            <div className="px-5 py-10 text-center text-xs text-neutral-600">还没有记录</div>
          ) : (
            <div className="divide-y divide-neutral-800/50">
              {txs.map((tx) => {
                const meta = TX_LABELS[tx.type] || {
                  label: tx.type,
                  icon: "fa-circle",
                  tone: "text-neutral-400",
                };
                return (
                  <div key={tx.id} className="flex items-center gap-3 px-5 py-2.5">
                    <i className={`fa-solid ${meta.icon} ${meta.tone} text-xs w-4 text-center`} />
                    <div className="flex-1 min-w-0">
                      <div className="text-xs text-neutral-300">{meta.label}</div>
                      <div className="text-[10px] text-neutral-600 truncate">{tx.reason}</div>
                    </div>
                    <div className="text-right shrink-0">
                      <div className={`text-xs ${tx.bytes >= 0 ? "text-brand-300" : "text-neutral-400"}`}>
                        {tx.bytes >= 0 ? "+" : "−"}
                        {formatBytes(Math.abs(tx.bytes))}
                      </div>
                      <div className="text-[10px] text-neutral-600">
                        {new Date(tx.created_at).toLocaleDateString("zh-CN")}
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}

          {txTotal > perPage && (
            <Pager page={page} perPage={perPage} total={txTotal} busy={txBusy} onPage={setPage} />
          )}
        </div>
      </div>
      <Footer />
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between">
      <dt className="text-neutral-600">{label}</dt>
      <dd className="text-neutral-300">{value}</dd>
    </div>
  );
}

function Center({ children }: { children: React.ReactNode }) {
  return <div className="min-h-screen flex items-center justify-center text-neutral-500">{children}</div>;
}

/**
 * Page navigation for the ledger.
 *
 * Shows a window of page numbers around the current one rather than all of
 * them — a ledger with 40 pages would otherwise render 40 buttons and wrap
 * across three lines.
 */
function Pager({
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
        title="上一页"
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
        title="下一页"
      >
        <i className="fa-solid fa-chevron-right text-[9px]" />
      </button>
    </div>
  );
}
