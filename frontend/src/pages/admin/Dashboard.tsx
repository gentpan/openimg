import { useEffect, useState } from "react";
import {
  ArcElement,
  BarElement,
  CategoryScale,
  Chart as ChartJS,
  Filler,
  Legend,
  LineElement,
  LinearScale,
  PointElement,
  Tooltip,
} from "chart.js";
import { Bar, Doughnut, Line } from "react-chartjs-2";
import { adminApi, formatBytes } from "../../api";
import type { Dashboard } from "../../types";
import { useChartTheme } from "../../chartTheme";

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Filler,
  Tooltip,
  Legend,
);

export default function AdminDashboard() {
  // Destructured under the same names the chart JSX below already uses, so
  // swapping the frozen constants for a live palette is a one-line change.
  const { ACCENT, ACCENT_DIM, SERIES, GRID, TICK, lineBase, barBase } = useChartTheme();
  const [data, setData] = useState<Dashboard | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    async function load() {
      try {
        const d = await adminApi.dashboard();
        if (alive) setData(d);
      } catch (e) {
        if (alive) setErr(e instanceof Error ? e.message : String(e));
      }
    }
    load();
    // The queue depth and backup backlog are the numbers you watch during an
    // incident, so this refreshes on its own rather than needing a reload.
    const t = setInterval(load, 30_000);
    return () => {
      alive = false;
      clearInterval(t);
    };
  }, []);

  if (err) return <div className="text-xs text-red-400">{err}</div>;
  if (!data) return <div className="text-xs text-neutral-600">加载中…</div>;

  const s = data.stats;
  const days = data.uploads_by_day;
  const dedupPct =
    s.stored_bytes + s.dedup_saved_bytes > 0
      ? (s.dedup_saved_bytes / (s.stored_bytes + s.dedup_saved_bytes)) * 100
      : 0;
  const compressionPct =
    s.original_bytes > 0 ? ((s.original_bytes - s.stored_bytes) / s.original_bytes) * 100 : 0;

  return (
    <div className="space-y-4">
      {/* KPI row */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <Kpi label="托管图片" value={s.total_images.toLocaleString()} sub={`今日 +${s.images_today}`} icon="fa-images" />
        <Kpi label="注册用户" value={s.total_users.toLocaleString()} sub={`今日签到 ${s.checkins_today}`} icon="fa-users" />
        <Kpi label="实际占用" value={formatBytes(s.stored_bytes)} sub={`已发放 ${formatBytes(s.granted_bytes)}`} icon="fa-database" />
        <Kpi
          label="队列 / 待备份"
          value={`${s.queue_depth} / ${s.pending_backup}`}
          sub={s.blocked_images > 0 ? `${s.blocked_images} 张已屏蔽` : "无屏蔽内容"}
          icon="fa-list-check"
          alert={s.queue_depth > 100 || s.pending_backup > 100}
        />
      </div>

      {/* Upload trend — dual axis: count on the left, bytes on the right. They
          diverge when users switch between screenshots and photos, which is
          exactly what you want to see. */}
      <Panel title="上传趋势" subtitle="最近 14 天">
        <div className="h-64">
          <Line
            data={{
              labels: days.map((d) => d.date.slice(5)),
              datasets: [
                {
                  label: "张数",
                  data: days.map((d) => d.uploads),
                  borderColor: ACCENT,
                  backgroundColor: ACCENT_DIM,
                  fill: true,
                  tension: 0.35,
                  pointRadius: 2,
                  yAxisID: "y",
                },
                {
                  label: "体积",
                  data: days.map((d) => d.bytes),
                  borderColor: SERIES[1],
                  backgroundColor: "transparent",
                  tension: 0.35,
                  pointRadius: 2,
                  yAxisID: "y1",
                },
              ],
            }}
            options={{
              ...lineBase,
              plugins: {
                ...lineBase.plugins,
                tooltip: {
                  callbacks: {
                    label: (c) =>
                      c.datasetIndex === 1
                        ? `体积: ${formatBytes(c.parsed.y ?? 0)}`
                        : `张数: ${c.parsed.y ?? 0}`,
                  },
                },
              },
              scales: {
                x: { grid: { color: GRID }, ticks: { color: TICK, font: { size: 10 } } },
                y: {
                  position: "left",
                  grid: { color: GRID },
                  ticks: { color: TICK, font: { size: 10 } },
                  beginAtZero: true,
                },
                y1: {
                  position: "right",
                  grid: { display: false },
                  beginAtZero: true,
                  ticks: {
                    color: TICK,
                    font: { size: 10 },
                    callback: (v) => formatBytes(Number(v), 0),
                  },
                },
              },
            }}
          />
        </div>
      </Panel>

      <div className="grid lg:grid-cols-3 gap-3">
        {/* Format mix */}
        <Panel title="格式分布" subtitle="近 30 天上传">
          <div className="h-56">
            {data.by_format.length === 0 ? (
              <Empty />
            ) : (
              <Doughnut
                data={{
                  labels: data.by_format.map((f) => f.ext.toUpperCase()),
                  datasets: [
                    {
                      data: data.by_format.map((f) => f.count),
                      backgroundColor: SERIES,
                      borderWidth: 0,
                      hoverOffset: 6,
                    },
                  ],
                }}
                options={{
                  cutout: "62%",
                  maintainAspectRatio: false,
                  plugins: {
                    legend: {
                      position: "bottom",
                      labels: { color: TICK, boxWidth: 10, boxHeight: 10, font: { size: 10 }, padding: 8 },
                    },
                  },
                }}
              />
            )}
          </div>
        </Panel>

        {/* Storage destinations */}
        <Panel title="存储去向" subtitle="按存储配置分布">
          <div className="h-56">
            {data.by_profile.length === 0 ? (
              <Empty />
            ) : (
              <Bar
                data={{
                  labels: data.by_profile.map((p) => p.name),
                  datasets: [
                    {
                      label: "占用",
                      data: data.by_profile.map((p) => p.bytes),
                      backgroundColor: data.by_profile.map((p) =>
                        p.kind === "platform" ? ACCENT : SERIES[1],
                      ),
                      borderRadius: 4,
                      barThickness: 22,
                    },
                  ],
                }}
                options={{
                  ...barBase,
                  indexAxis: "y" as const,
                  plugins: {
                    legend: { display: false },
                    tooltip: {
                      callbacks: {
                        label: (c) => {
                          const p = data.by_profile[c.dataIndex];
                          return `${formatBytes(c.parsed.x ?? 0)} · ${p.count} 张`;
                        },
                      },
                    },
                  },
                  scales: {
                    x: {
                      grid: { color: GRID },
                      ticks: { color: TICK, font: { size: 10 }, callback: (v) => formatBytes(Number(v), 0) },
                      beginAtZero: true,
                    },
                    y: { grid: { display: false }, ticks: { color: TICK, font: { size: 10 } } },
                  },
                }}
              />
            )}
          </div>
        </Panel>

        {/* Space efficiency: what dedup and re-compression bought */}
        <Panel title="空间效率" subtitle="去重与压缩省下的空间">
          <div className="h-56 flex flex-col justify-center gap-4 px-1">
            <Gauge
              label="去重节省"
              pct={dedupPct}
              detail={`${formatBytes(s.dedup_saved_bytes)} / 引用总量 ${formatBytes(
                s.stored_bytes + s.dedup_saved_bytes,
              )}`}
              color={ACCENT}
            />
            <Gauge
              label="压缩节省"
              pct={compressionPct}
              detail={`原始 ${formatBytes(s.original_bytes)} → 存储 ${formatBytes(s.stored_bytes)}`}
              color={SERIES[1]}
            />
            <Gauge
              label="配额兑现率"
              pct={s.granted_bytes > 0 ? (s.stored_bytes / s.granted_bytes) * 100 : 0}
              detail={`已发放 ${formatBytes(s.granted_bytes)}，实际用掉 ${formatBytes(s.stored_bytes)}`}
              color={SERIES[2]}
            />
          </div>
        </Panel>
      </div>

      {/* Top users by space */}
      <Panel title="空间占用 Top 10" subtitle="已用 vs 已获得配额">
        <div className="h-72">
          {data.top_users_by_space.length === 0 ? (
            <Empty />
          ) : (
            <Bar
              data={{
                labels: data.top_users_by_space.map((u) => maskEmail(u.email)),
                datasets: [
                  {
                    label: "已使用",
                    data: data.top_users_by_space.map((u) => u.used_bytes),
                    backgroundColor: ACCENT,
                    borderRadius: 4,
                  },
                  {
                    label: "总配额",
                    data: data.top_users_by_space.map((u) => u.quota_bytes),
                    backgroundColor: ACCENT_DIM,
                    borderRadius: 4,
                  },
                ],
              }}
              options={{
                ...barBase,
                plugins: {
                  ...barBase.plugins,
                  tooltip: {
                    callbacks: {
                      label: (c) => {
                        const u = data.top_users_by_space[c.dataIndex];
                        return `${c.dataset.label}: ${formatBytes(c.parsed.y ?? 0)} · ${u.image_count} 张`;
                      },
                    },
                  },
                },
                scales: {
                  x: { grid: { display: false }, ticks: { color: TICK, font: { size: 9 } } },
                  y: {
                    grid: { color: GRID },
                    ticks: { color: TICK, font: { size: 10 }, callback: (v) => formatBytes(Number(v), 0) },
                    beginAtZero: true,
                  },
                },
              }}
            />
          )}
        </div>
      </Panel>

      {/* Ops tables */}
      <div className="grid lg:grid-cols-2 gap-3">
        <Panel title="最近登录" subtitle={`当前活跃会话 ${data.active_sessions}`}>
          <Table
            head={["邮箱", "IP", "时间"]}
            rows={data.recent_logins.map((l) => [
              maskEmail(l.email),
              l.ip,
              new Date(l.login_at).toLocaleString("zh-CN", { hour12: false }),
            ])}
            empty="暂无登录记录"
          />
        </Panel>

        <Panel title="备份失败" subtitle={data.backup_failures.length ? "需要关注" : "一切正常"}>
          <Table
            head={["文件", "错误", "时间"]}
            rows={data.backup_failures.map((f) => [
              f.orig_name,
              f.error,
              new Date(f.created_at).toLocaleString("zh-CN", { hour12: false }),
            ])}
            empty="没有失败的备份任务"
          />
        </Panel>
      </div>
    </div>
  );
}

function Kpi({
  label,
  value,
  sub,
  icon,
  alert,
}: {
  label: string;
  value: string;
  sub?: string;
  icon: string;
  alert?: boolean;
}) {
  return (
    <div
      className={`rounded-2xl border bg-neutral-900/40 px-4 py-3.5 ${
        alert ? "border-amber-500/40" : "border-neutral-800"
      }`}
    >
      <div className="flex items-center gap-1.5 text-[10px] text-neutral-500">
        <i className={`fa-solid ${icon}`} />
        {label}
      </div>
      <div className={`mt-1 text-xl font-brand ${alert ? "text-amber-300" : "text-neutral-100"}`}>{value}</div>
      {sub && <div className="mt-0.5 text-[10px] text-neutral-600">{sub}</div>}
    </div>
  );
}

function Panel({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-4">
      <div className="mb-3">
        <div className="text-xs text-neutral-300">{title}</div>
        {subtitle && <div className="text-[10px] text-neutral-600 mt-0.5">{subtitle}</div>}
      </div>
      {children}
    </div>
  );
}

function Gauge({ label, pct, detail, color }: { label: string; pct: number; detail: string; color: string }) {
  const clamped = Math.max(0, Math.min(100, pct));
  return (
    <div>
      <div className="flex items-baseline justify-between mb-1.5">
        <span className="text-[11px] text-neutral-400">{label}</span>
        <span className="text-sm font-brand" style={{ color }}>
          {clamped.toFixed(1)}%
        </span>
      </div>
      <div className="h-1.5 rounded-full bg-neutral-800 overflow-hidden">
        <div
          className="h-full rounded-full transition-all duration-500"
          style={{ width: `${clamped}%`, backgroundColor: color }}
        />
      </div>
      <div className="mt-1 text-[10px] text-neutral-600">{detail}</div>
    </div>
  );
}

function Table({ head, rows, empty }: { head: string[]; rows: string[][]; empty: string }) {
  if (rows.length === 0) {
    return <div className="py-10 text-center text-xs text-neutral-600">{empty}</div>;
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left">
        <thead>
          <tr className="text-[10px] text-neutral-600">
            {head.map((h) => (
              <th key={h} className="pb-2 font-normal">
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-neutral-800/50">
          {rows.map((r, i) => (
            <tr key={i} className="text-[11px] text-neutral-400">
              {r.map((cell, j) => (
                <td key={j} className="py-2 pr-3 max-w-[14rem] truncate" title={cell}>
                  {cell}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Empty() {
  return <div className="h-full flex items-center justify-center text-xs text-neutral-600">暂无数据</div>;
}

/** Masks an email for the admin view — enough to identify, not to shoulder-surf. */
function maskEmail(e: string): string {
  const at = e.indexOf("@");
  if (at <= 1) return e;
  return e.slice(0, Math.min(3, at)) + "***" + e.slice(at);
}
