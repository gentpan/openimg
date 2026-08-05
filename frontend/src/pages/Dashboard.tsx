import { useEffect, useMemo, useState } from "react";
import { Link, Navigate } from "react-router-dom";
import { useAuth } from "../AuthContext";
import Footer from "../Footer";
import Nav from "../components/Nav";
import { formatBytes, imageApi, quotaApi } from "../api";
import { AreaSpark, useCountUp } from "../components/Meters";
import ActivityCalendar from "../components/ActivityCalendar";
import ArcGauge, { CategoryBars } from "../components/ArcGauge";
import type { Image, QuotaInfo, QuotaTransaction } from "../types";
import { useChartTheme } from "../chartTheme";



/**
 * The signed-in landing page: what you have, what you've used, and one click
 * to the thing you came to do. Deliberately read-only — every action here is
 * a link to the page that owns it.
 */
export default function DashboardPage() {
  const { SERIES, FORMATS, SAVED } = useChartTheme();
  const { user, loading } = useAuth();
  const [quota, setQuota] = useState<QuotaInfo | null>(null);
  const [txs, setTxs] = useState<QuotaTransaction[]>([]);
  const [recent, setRecent] = useState<Image[]>([]);
  const [summary, setSummary] = useState<Awaited<ReturnType<typeof quotaApi.storageSummary>> | null>(null);

  useEffect(() => {
    if (!user) return;
    Promise.all([
      quotaApi.info(),
      quotaApi.transactions(200),
      imageApi.list({ limit: 12 }),
      quotaApi.storageSummary(),
    ])
      .then(([q, t, imgs, sum]) => {
        setQuota(q);
        setTxs(t.transactions);
        setRecent(imgs.images);
        setSummary(sum);
      })
      .catch(() => {});
  }, [user]);

  // Space earned per day over the last 30 days, from the ledger.
  const earnSeries = useMemo(() => {
    const byDay = new Map<string, number>();
    for (let i = 29; i >= 0; i--) {
      const d = new Date();
      d.setUTCDate(d.getUTCDate() - i);
      byDay.set(d.toISOString().slice(0, 10), 0);
    }
    for (const tx of txs) {
      if (tx.bytes <= 0) continue;
      const day = tx.created_at.slice(0, 10);
      if (byDay.has(day)) byDay.set(day, (byDay.get(day) || 0) + tx.bytes);
    }
    return Array.from(byDay.entries());
  }, [txs]);

  // Hooks must run before the early returns below.
  const usedAnimated = useCountUp(quota?.used_bytes ?? 0, 1000);

  if (loading) return <Center>加载中…</Center>;
  if (!user) return <Navigate to="/login" replace />;

  const usedPct = quota && quota.quota_bytes > 0 ? (quota.used_bytes / quota.quota_bytes) * 100 : 0;
  const earnedTotal = earnSeries.reduce((n, [, v]) => n + v, 0);

  // Where the stored bytes actually go — measured per object, not estimated.

  return (
    <div className="min-h-screen flex flex-col bg-neutral-950">
      <Nav />
      <div className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 py-8">
        <div className="flex flex-wrap items-center gap-3 mb-5">
          <div>
            <h1 className="text-lg font-brand text-neutral-100">
              你好，{user.name || user.email.split("@")[0]}
            </h1>
            <p className="text-xs text-neutral-600 mt-0.5">
              {quota?.tier.description || "欢迎回来"}
              {user.checked_in_today && ` · 已连续签到 ${user.checkin_streak} 天`}
            </p>
          </div>
          <div className="flex-1" />
          <Link
            to="/upload"
            className="rounded-lg bg-brand-600 px-3.5 py-1.5 text-xs font-medium text-brand-ink hover:bg-brand-500 transition"
          >
            <i className="fa-solid fa-cloud-arrow-up mr-1.5" />
            上传图片
          </Link>
        </div>

        {/* KPI row */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-4">
          <Kpi
            label="可用空间"
            value={formatBytes(quota?.available_bytes ?? user.available_bytes, 0)}
            sub={`已用 ${formatBytes(usedAnimated, 1)} · ${usedPct.toFixed(0)}% · 共 ${formatBytes(quota?.quota_bytes ?? user.quota_bytes, 0)}`}
            icon="fa-database"
            alert={usedPct >= 90}
            href="/space"
          />
          <Kpi
            label="图片总数"
            value={String(quota?.image_count ?? 0)}
            sub={`今日上传 ${quota?.uploads_today ?? 0} / ${quota?.tier.daily_upload_count ?? 0}`}
            icon="fa-images"
          />
          <Kpi
            label="连续签到"
            value={`${user.checkin_streak} 天`}
            sub={user.checked_in_today ? "今日已签到" : "今天还没签到"}
            icon="fa-calendar-check"
          />
          <Kpi
            label="压缩省下"
            value={formatBytes(Math.max(0, (summary?.size_orig ?? 0) - (summary?.size_stored ?? 0)), 0)}
            sub={
              summary && summary.size_orig > 0
                ? `原始 ${formatBytes(summary.size_orig, 0)} → 存储 ${formatBytes(summary.size_stored, 0)}`
                : "全部图片累计"
            }
            icon="fa-compress"
          />
        </div>

        <div className="mb-4">
        {/* Space earned trend — full width now that the usage card it sat beside
            has gone; that card repeated /space and the KPI above it. */}
        <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-5">
          <div className="flex items-baseline justify-between mb-3">
            <div>
              <div className="text-[10px] uppercase tracking-wider text-neutral-600">空间获得趋势</div>
              <div className="text-xs text-neutral-500 mt-1">最近 30 天 · 签到 / 邀请 / 赠送</div>
            </div>
            <div className="text-lg font-brand text-brand-400 tabular-nums">
              +{formatBytes(earnedTotal, 0)}
            </div>
          </div>
          <AreaSpark
            data={earnSeries.map(([d, v]) => ({ label: d.slice(5), value: v }))}
            height={140}
            formatValue={(n) => formatBytes(n, 1)}
          />
        </div>
        </div>

        <div className="grid lg:grid-cols-2 gap-3 mb-4">
          <Panel
            title="压缩效果"
            subtitle={summary ? `全部 ${summary.images} 张图片` : "统计中…"}
          >
            {summary && summary.size_orig > 0 ? (
              <>
                <ArcGauge
                  segments={[
                    { label: "实际占用", value: summary.size_stored, color: SERIES[1] },
                    { label: "已省下", value: Math.max(0, summary.size_orig - summary.size_stored), color: SAVED },
                  ]}
                  formatValue={(v) => formatBytes(v, 1)}
                  className="py-2"
                />
                {/* The old three-way split, demoted: it describes how we store
                    a picture, which is our business, not the user's. */}
                <div className="mt-3 border-t border-neutral-800/60 pt-2 text-[10px] text-neutral-600 leading-relaxed">
                  占用明细 · 主图 {formatBytes(summary.size_primary, 0)} · 全尺寸变体{" "}
                  {formatBytes(summary.size_variants, 0)} · 缩略图 {formatBytes(summary.size_thumbs, 0)}
                  {summary.size_unclassified > 0 && (
                    <> · 早期未拆分 {formatBytes(summary.size_unclassified, 0)}</>
                  )}
                </div>
              </>
            ) : (
              <Empty />
            )}
          </Panel>

          <Panel title="存储分布" subtitle="平台空间与你自己绑定的存储">
            {summary && summary.by_profile.length > 0 ? (
              <>
                <CategoryBars
                  rows={summary.by_profile.map((p, i) => ({
                    label: p.name,
                    value: p.bytes,
                    note: `${p.images} 张 · ${formatBytes(p.bytes, 0)}`,
                    color: FORMATS[i % FORMATS.length],
                  }))}
                  className="py-1"
                />
                {/* With only the platform pool the chart above is a single full
                    bar, which says nothing. The format split fills the panel so
                    it still earns its space until a second store is bound. */}
                {summary.by_profile.length === 1 && summary.by_format.length > 0 && (
                  <div className="mt-3 border-t border-neutral-800/60 pt-3">
                    <div className="text-[10px] text-neutral-600 mb-2">按格式</div>
                    <CategoryBars
                      rows={summary.by_format.map((f, i) => ({
                        label: f.ext.toUpperCase(),
                        value: f.bytes,
                        note: `${f.images} 张 · ${formatBytes(f.bytes, 0)}`,
                        color: FORMATS[i % FORMATS.length],
                      }))}
                    />
                  </div>
                )}
              </>
            ) : (
              <Empty />
            )}
          </Panel>
        </div>

        <Panel title="上传活动" subtitle="最近 30 天 · 悬停查看当天上传张数与占用" className="mb-4">
          <ActivityCalendar transactions={txs} days={30} columns={30} />
        </Panel>

        {/* Recent uploads */}
        <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-4">
          <div className="flex items-center gap-3 mb-3">
            <div>
              <div className="text-xs text-neutral-300">最近上传</div>
              <div className="text-[10px] text-neutral-600 mt-0.5">点击进入图库管理</div>
            </div>
            <div className="flex-1" />
            <Link to="/gallery" className="text-[11px] text-brand-400 hover:underline">
              查看全部 →
            </Link>
          </div>
          {recent.length === 0 ? (
            <div className="py-10 text-center">
              <div className="text-xs text-neutral-600 mb-2">还没有上传过图片</div>
              <Link to="/upload" className="text-xs text-brand-400 hover:underline">
                去上传第一张 →
              </Link>
            </div>
          ) : (
            <div className="grid grid-cols-4 sm:grid-cols-6 lg:grid-cols-12 gap-2">
              {recent.map((img) => (
                <Link
                  key={img.id}
                  to="/gallery"
                  title={`${img.orig_name} · ${formatBytes(img.size_stored, 0)}`}
                  className="rounded-lg overflow-hidden border border-neutral-800 hover:border-brand-500 transition"
                >
                  <img
                    src={img.thumb_url}
                    alt={img.orig_name}
                    loading="lazy"
                    className="w-full aspect-square object-cover"
                  />
                </Link>
              ))}
            </div>
          )}
        </div>
      </div>
      <Footer />
    </div>
  );
}

function Kpi({
  label,
  value,
  sub,
  icon,
  alert,
  href,
}: {
  label: string;
  value: string;
  sub?: string;
  icon: string;
  alert?: boolean;
  /// When set the whole card is the link to the page that owns this number.
  href?: string;
}) {
  const body = (
    <>
      <div className="flex items-center gap-1.5 text-[10px] text-neutral-500">
        <i className={`fa-solid ${icon}`} />
        {label}
        {href && <i className="fa-solid fa-arrow-right ml-auto text-[9px] opacity-0 group-hover:opacity-60 transition" />}
      </div>
      <div className={`mt-1 text-xl font-brand ${alert ? "text-amber-300" : "text-neutral-100"}`}>{value}</div>
      {sub && <div className="mt-0.5 text-[10px] text-neutral-600">{sub}</div>}
    </>
  );
  const shell = `rounded-2xl border bg-neutral-900/40 px-4 py-3.5 ${
    alert ? "border-amber-500/40" : "border-neutral-800"
  }`;
  // With the space-usage panel gone, this card is the only place the overview
  // reports quota, so it also has to carry the way through to the page that
  // owns it.
  return href ? (
    <Link to={href} className={`group block transition hover:border-neutral-700 ${shell}`}>
      {body}
    </Link>
  ) : (
    <div className={shell}>{body}</div>
  );
}

function Panel({
  title,
  subtitle,
  children,
  className = "",
}: {
  title: string;
  subtitle?: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={`rounded-2xl border border-neutral-800 bg-neutral-900/40 p-4 ${className}`}>
      <div className="mb-3">
        <div className="text-xs text-neutral-300">{title}</div>
        {subtitle && <div className="text-[10px] text-neutral-600 mt-0.5">{subtitle}</div>}
      </div>
      {children}
    </div>
  );
}

function Empty() {
  return <div className="h-full flex items-center justify-center text-xs text-neutral-600">暂无数据</div>;
}

function Center({ children }: { children: React.ReactNode }) {
  return <div className="min-h-screen flex items-center justify-center text-neutral-500">{children}</div>;
}
