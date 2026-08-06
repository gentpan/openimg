import { useEffect, useState, useCallback } from "react";
import { Link } from "react-router-dom";
import Nav from "../components/Nav";
import Footer from "../Footer";
import AdminDashboard from "./admin/Dashboard";
import LoginMethods from "./admin/LoginMethods";
import UploadSettings from "./admin/UploadSettings";
import { adminApi, formatBytes, imageApi, reportCategoryLabel } from "../api";
import type { AdminImage, AdminQuotaTx, AdminUser, Report, UserGroup } from "../types";
import Pager from "../components/Pager";
import PageSizeMenu from "../components/PageSizeMenu";

type Tab = "dashboard" | "users" | "groups" | "images" | "reports" | "ledger" | "login" | "upload";

const TABS: { key: Tab; label: string; icon: string }[] = [
  { key: "dashboard", label: "总览", icon: "fa-chart-line" },
  { key: "users", label: "用户", icon: "fa-users" },
  { key: "groups", label: "用户组", icon: "fa-layer-group" },
  { key: "images", label: "图片", icon: "fa-images" },
  { key: "reports", label: "举报", icon: "fa-flag" },
  { key: "ledger", label: "空间流水", icon: "fa-receipt" },
  { key: "login", label: "登录方式", icon: "fa-right-to-bracket" },
  { key: "upload", label: "上传设置", icon: "fa-sliders" },
];

export default function AdminLayout() {
  const [tab, setTab] = useState<Tab>("dashboard");
  const [openReports, setOpenReports] = useState(0);

  useEffect(() => {
    adminApi
      .listReports("open")
      .then((r) => setOpenReports(r.length))
      .catch(() => {});
  }, [tab]);

  return (
    <div className="min-h-screen flex flex-col bg-neutral-950">
      <Nav />
      <div className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 py-8">
        <div className="flex items-center gap-3 mb-5">
          <h1 className="flex items-center gap-2.5 text-lg font-brand text-neutral-100">
            <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-neutral-900 text-brand-400">
              <i className="fa-solid fa-shield-halved text-sm" />
            </span>
            管理后台
          </h1>
          <div className="flex-1" />
          <Link to="/" className="text-xs text-neutral-500 hover:text-brand-300">
            返回站点 →
          </Link>
        </div>

        <div className="flex gap-1 mb-5 overflow-x-auto pb-1">
          {TABS.map((t) => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={`shrink-0 flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium transition ${
                tab === t.key
                  ? "bg-brand-600 text-brand-ink"
                  : "bg-neutral-900 text-neutral-400 hover:text-neutral-100 hover:bg-neutral-800"
              }`}
            >
              <i className={`fa-solid ${t.icon} text-[10px]`} />
              {t.label}
              {t.key === "reports" && openReports > 0 && (
                <span className="ml-0.5 rounded-full bg-red-600 px-1.5 text-[9px] text-white">
                  {openReports}
                </span>
              )}
            </button>
          ))}
        </div>

        {tab === "dashboard" && <AdminDashboard />}
        {tab === "users" && <UsersTab />}
        {tab === "groups" && <GroupsTab />}
        {tab === "images" && <ImagesTab />}
        {tab === "reports" && <ReportsTab onChange={setOpenReports} />}
        {tab === "ledger" && <LedgerTab />}
        {tab === "login" && <LoginMethods />}
        {tab === "upload" && <UploadSettings />}
      </div>
      <Footer />
    </div>
  );
}

/* ---------- Users ---------- */

function UsersTab() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [groups, setGroups] = useState<UserGroup[]>([]);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<string | null>(null);

  async function load() {
    const [u, g] = await Promise.all([adminApi.listUsers(), adminApi.listGroups()]);
    setUsers(u);
    setGroups(g);
  }
  useEffect(() => {
    load().catch(() => {});
  }, []);

  async function patch(id: string, p: Record<string, string>) {
    setBusy(true);
    try {
      await adminApi.updateUser(id, p);
      await load();
    } finally {
      setBusy(false);
    }
  }

  async function grant(u: AdminUser) {
    const mb = prompt(`给 ${u.email} 增加多少 MB 空间？（负数为扣减）`, "100");
    if (!mb) return;
    const n = Number(mb);
    if (!Number.isFinite(n) || n === 0) return;
    setBusy(true);
    try {
      await adminApi.adjustQuota(u.id, Math.round(n * 1024 * 1024));
      await load();
      setMsg(`已调整 ${u.email} 的空间 ${n > 0 ? "+" : ""}${n} MB`);
    } catch (e) {
      setMsg(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  async function ban(u: AdminUser) {
    const purge = confirm(
      `确定封禁 ${u.email}？\n\n点「确定」= 封禁并删除其全部 ${u.image_count} 张图片\n点「取消」后可选择仅封禁`,
    );
    if (!purge && !confirm(`仅封禁 ${u.email}，保留其图片？`)) return;
    setBusy(true);
    try {
      const res = await adminApi.banUser(u.id, purge);
      await load();
      setMsg(`已封禁 ${u.email}${res.purged ? `，删除 ${res.purged} 张图片` : ""}`);
    } catch (e) {
      setMsg(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card>
      {msg && <div className="mb-3 text-xs text-brand-300">{msg}</div>}
      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead className="text-[10px] text-neutral-600">
            <tr>
              {["邮箱", "用户组", "图片", "空间", "状态", "注册时间 / IP", "最后登录 / IP", ""].map((h) => (
                <th key={h} className="pb-2 font-normal whitespace-nowrap pr-3">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-neutral-800/50">
            {users.map((u) => (
              <tr key={u.id} className="text-neutral-400">
                <td className="py-2 pr-3">
                  <div className="text-neutral-200">{u.email}</div>
                  {u.role === "admin" && <span className="text-[9px] text-brand-400">管理员</span>}
                </td>
                <td className="py-2 pr-3">
                  <select
                    value={u.group_id || ""}
                    disabled={busy}
                    onChange={(e) => patch(u.id, { group_id: e.target.value })}
                    className="rounded-md bg-neutral-800 px-2 py-1 text-[11px] outline-none border border-transparent focus:border-brand-500"
                  >
                    <option value="">无</option>
                    {groups.map((g) => (
                      <option key={g.id} value={g.id}>
                        {g.name}
                      </option>
                    ))}
                  </select>
                </td>
                <td className="py-2 pr-3 whitespace-nowrap">{u.image_count}</td>
                <td className="py-2 pr-3 whitespace-nowrap">
                  {formatBytes(u.used_bytes, 0)} / {formatBytes(u.quota_bytes, 0)}
                </td>
                <td className="py-2 pr-3">
                  <span className={u.status === "active" ? "text-teal-400" : "text-red-400"}>
                    {u.status === "active" ? "正常" : "已停用"}
                  </span>
                </td>
                <td className="py-2 pr-3 whitespace-nowrap text-[10px]">
                  <div>{new Date(u.created_at).toLocaleString("zh-CN", { hour12: false })}</div>
                  <div className="text-neutral-600 font-mono">{u.signup_ip || "—"}</div>
                </td>
                <td className="py-2 pr-3 whitespace-nowrap text-[10px]">
                  <div>
                    {u.last_login_at
                      ? new Date(u.last_login_at).toLocaleString("zh-CN", { hour12: false })
                      : "从未登录"}
                  </div>
                  <div className="text-neutral-600 font-mono">{u.last_login_ip || "—"}</div>
                </td>
                <td className="py-2 whitespace-nowrap">
                  <button onClick={() => grant(u)} disabled={busy} className="text-brand-400 hover:underline mr-2">
                    调整空间
                  </button>
                  {u.status === "active" ? (
                    <button onClick={() => ban(u)} disabled={busy} className="text-red-400 hover:underline">
                      封禁
                    </button>
                  ) : (
                    <button
                      onClick={() => patch(u.id, { status: "active" })}
                      disabled={busy}
                      className="text-teal-400 hover:underline"
                    >
                      恢复
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}

/* ---------- Groups ---------- */

const GROUP_FIELDS: { key: keyof UserGroup; label: string; unit: "bytes" | "count" }[] = [
  { key: "max_file_size", label: "单文件上限", unit: "bytes" },
  { key: "daily_upload_count", label: "每日上传数", unit: "count" },
  { key: "signup_space", label: "注册赠送", unit: "bytes" },
  { key: "checkin_min_space", label: "签到下限", unit: "bytes" },
  { key: "checkin_max_space", label: "签到上限", unit: "bytes" },
  { key: "streak_bonus_space", label: "连续奖励", unit: "bytes" },
  { key: "streak_bonus_days", label: "连续天数门槛", unit: "count" },
  { key: "referral_space", label: "邀请奖励", unit: "bytes" },
  { key: "max_total_space", label: "空间上限(0=无限)", unit: "bytes" },
  { key: "max_profiles", label: "自有存储数量", unit: "count" },
];

function GroupsTab() {
  const [groups, setGroups] = useState<UserGroup[]>([]);
  const [busy, setBusy] = useState(false);

  async function load() {
    setGroups(await adminApi.listGroups());
  }
  useEffect(() => {
    load().catch(() => {});
  }, []);

  async function save(g: UserGroup, key: keyof UserGroup, raw: string, unit: "bytes" | "count") {
    const n = Number(raw);
    if (!Number.isFinite(n) || n < 0) return;
    setBusy(true);
    try {
      await adminApi.updateGroup(g.id, {
        [key]: unit === "bytes" ? Math.round(n * 1024 * 1024) : n,
      } as Partial<UserGroup>);
      await load();
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid lg:grid-cols-3 gap-3">
      {groups.map((g) => (
        <Card key={g.id}>
          <div className="mb-3">
            <div className="text-sm text-neutral-100">{g.name}</div>
            <div className="text-[10px] text-neutral-600">{g.description}</div>
          </div>
          <div className="space-y-2">
            {GROUP_FIELDS.map((f) => (
              <div key={f.key} className="flex items-center justify-between gap-2">
                <span className="text-[11px] text-neutral-500 shrink-0">{f.label}</span>
                <div className="flex items-center gap-1">
                  <input
                    type="number"
                    defaultValue={
                      f.unit === "bytes" ? Math.round((g[f.key] as number) / 1024 / 1024) : (g[f.key] as number)
                    }
                    disabled={busy}
                    onBlur={(e) => save(g, f.key, e.target.value, f.unit)}
                    className="w-24 rounded-md bg-neutral-800 px-2 py-1 text-[11px] text-right outline-none border border-transparent focus:border-brand-500"
                  />
                  <span className="text-[10px] text-neutral-600 w-6">{f.unit === "bytes" ? "MB" : ""}</span>
                </div>
              </div>
            ))}
            <div className="flex items-center justify-between gap-2 pt-1">
              <span className="text-[11px] text-neutral-500">允许绑定自有存储</span>
              <input
                type="checkbox"
                defaultChecked={g.allow_byos}
                disabled={busy}
                onChange={async (e) => {
                  setBusy(true);
                  try {
                    await adminApi.updateGroup(g.id, { allow_byos: e.target.checked });
                    await load();
                  } finally {
                    setBusy(false);
                  }
                }}
                className="accent-brand-800"
              />
            </div>
            <div className="flex items-center justify-between gap-2">
              <span className="text-[11px] text-neutral-500 shrink-0">允许格式</span>
              <input
                defaultValue={g.allowed_formats}
                disabled={busy}
                onBlur={async (e) => {
                  setBusy(true);
                  try {
                    await adminApi.updateGroup(g.id, { allowed_formats: e.target.value });
                    await load();
                  } finally {
                    setBusy(false);
                  }
                }}
                className="w-40 rounded-md bg-neutral-800 px-2 py-1 text-[11px] outline-none border border-transparent focus:border-brand-500"
              />
            </div>
          </div>
        </Card>
      ))}
    </div>
  );
}

/* ---------- Images ---------- */

function ImagesTab() {
  const [images, setImages] = useState<AdminImage[]>([]);
  const [status, setStatus] = useState("");
  const [detail, setDetail] = useState<AdminImage | null>(null);

  async function load() {
    const next = await adminApi.listImages({ status: status || undefined, limit: 60 });
    setImages(next);
    // Keep the open panel pointed at fresh data, or close it if the image is
    // gone — a stale panel is how you block something twice.
    setDetail((d) => (d ? next.find((i) => i.id === d.id) ?? null : null));
  }
  useEffect(() => {
    load().catch(() => {});
  }, [status]);

  async function toggleBlock(img: AdminImage) {
    await adminApi.blockImage(img.id);
    await load();
  }

  async function removeImage(img: AdminImage) {
    if (!confirm(`确定删除这张图片？\n\n${img.orig_name}\n上传者：${img.owner_email}\n\n对象会被清除，占用的 ${formatBytes(img.size_stored, 0)} 退还给该用户。此操作不可撤销。`)) return;
    await imageApi.remove(img.id);
    setDetail(null);
    await load();
  }

  return (
    <Card>
      <div className="flex items-center gap-2 mb-4">
        {[
          { v: "", l: "全部" },
          { v: "active", l: "正常" },
          { v: "blocked", l: "已屏蔽" },
        ].map((o) => (
          <button
            key={o.v}
            onClick={() => setStatus(o.v)}
            className={`px-2.5 py-1 rounded-md text-xs transition ${
              status === o.v
                ? "bg-brand-600/20 text-brand-300 border border-brand-500/30"
                : "bg-neutral-800 text-neutral-400 hover:text-neutral-100 border border-transparent"
            }`}
          >
            {o.l}
          </button>
        ))}
      </div>
      <div className="grid grid-cols-3 sm:grid-cols-5 lg:grid-cols-8 gap-2">
        {images.map((img) => (
          <button
            key={img.id}
            onClick={() => setDetail(img)}
            title={`${img.orig_name}\n${img.owner_email}`}
            className={`group relative block rounded-xl overflow-hidden border transition ${
              detail?.id === img.id ? "border-brand-500" : "border-neutral-800 hover:border-neutral-700"
            }`}
          >
            <img src={img.thumb_url} alt="" loading="lazy" className="w-full aspect-square object-cover" />
            {img.status === "blocked" && (
              <div className="absolute inset-0 bg-red-950/60 flex items-center justify-center">
                <i className="fa-solid fa-eye-slash text-red-300" />
              </div>
            )}
            {/* The uploader on the tile itself, not only in the panel: scanning
                a grid for one account's images should not take 60 clicks. */}
            <span className="absolute inset-x-0 bottom-0 scrim px-1.5 py-1 text-left text-[9px] leading-tight text-neutral-300 truncate">
              {img.owner_name || img.owner_email}
            </span>
          </button>
        ))}
      </div>
      {images.length === 0 && <div className="py-10 text-center text-xs text-neutral-600">暂无图片</div>}

      {detail && (
        <AdminImageDetail
          img={detail}
          onClose={() => setDetail(null)}
          onBlock={() => toggleBlock(detail)}
          onDelete={() => removeImage(detail)}
        />
      )}
    </Card>
  );
}

/**
 * Moderation panel. Blocking or deleting a picture is a decision about an
 * account as much as about a file, so the uploader sits at the top with a
 * one-click jump to the rest of their images.
 */
function AdminImageDetail({
  img,
  onClose,
  onBlock,
  onDelete,
}: {
  img: AdminImage;
  onClose: () => void;
  onBlock: () => void;
  onDelete: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    try {
      await fn();
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/60 p-0 sm:p-4" onClick={onClose}>
      <div
        onClick={(e) => e.stopPropagation()}
        className="shadow-panel flex max-h-[90vh] w-full max-w-lg flex-col overflow-hidden rounded-t-2xl sm:rounded-2xl border border-neutral-800 bg-neutral-900"
      >
        <div className="flex items-center gap-2 border-b border-neutral-800/60 px-5 py-3.5">
          <span className="text-sm text-neutral-100">图片详情</span>
          {img.status === "blocked" && (
            <span className="rounded-full bg-red-900/50 px-2 py-0.5 text-[10px] text-red-300">已屏蔽</span>
          )}
          <div className="flex-1" />
          <button onClick={onClose} className="text-neutral-500 hover:text-neutral-100 transition">
            <i className="fa-solid fa-xmark" />
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          <img src={img.thumb_url} alt="" className="mb-4 w-full rounded-xl bg-neutral-950 object-contain max-h-56" />

          <div className="mb-3 rounded-xl border border-brand-900/40 bg-brand-950/20 px-3 py-2.5">
            <div className="text-[10px] text-neutral-500">上传者</div>
            <div className="mt-0.5 text-xs text-neutral-100 break-all">
              {img.owner_name ? `${img.owner_name} · ` : ""}
              {img.owner_email}
            </div>
            <a
              href={`/admin?tab=images&user=${img.owner_id}`}
              onClick={(e) => {
                e.preventDefault();
                navigator.clipboard?.writeText(img.owner_email);
              }}
              className="mt-1 inline-block text-[10px] text-brand-400 hover:underline"
            >
              <i className="fa-solid fa-copy mr-1" />
              复制邮箱
            </a>
          </div>

          <dl className="space-y-1 text-[11px]">
            <DetailRow label="文件名" value={img.orig_name} />
            <DetailRow label="尺寸" value={`${img.width} × ${img.height}`} />
            <DetailRow label="存储占用" value={formatBytes(img.size_stored)} />
            <DetailRow label="格式" value={img.ext.toUpperCase()} />
            <DetailRow label="短链" value={img.short_code || "—"} />
            <DetailRow label="上传时间" value={new Date(img.created_at).toLocaleString("zh-CN")} />
          </dl>
        </div>

        <div className="flex items-center gap-2 border-t border-neutral-800/60 px-5 py-3">
          <button
            onClick={() => run(async () => onBlock())}
            disabled={busy}
            className="rounded-lg bg-neutral-800 px-3 py-1.5 text-xs text-neutral-200 hover:bg-neutral-700 disabled:opacity-50 transition"
          >
            <i className={`fa-solid ${img.status === "blocked" ? "fa-eye" : "fa-eye-slash"} mr-1.5`} />
            {img.status === "blocked" ? "解除屏蔽" : "屏蔽"}
          </button>
          <div className="flex-1" />
          <button
            onClick={() => run(async () => onDelete())}
            disabled={busy}
            className="rounded-lg bg-red-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-red-700 disabled:opacity-50 transition"
          >
            <i className="fa-solid fa-trash-can mr-1.5" />
            删除
          </button>
        </div>
      </div>
    </div>
  );
}

/* ---------- Reports ---------- */

function ReportsTab({ onChange }: { onChange: (n: number) => void }) {
  const [reports, setReports] = useState<Report[]>([]);
  const [status, setStatus] = useState("open");
  const [busy, setBusy] = useState(false);
  const [detail, setDetail] = useState<Report | null>(null);

  async function load() {
    const r = await adminApi.listReports(status);
    setReports(r);
    if (status === "open") onChange(r.length);
  }
  useEffect(() => {
    load().catch(() => {});
  }, [status]);

  async function resolve(r: Report, action: "dismiss" | "block" | "block_and_ban") {
    const labels = { dismiss: "驳回", block: "屏蔽图片", block_and_ban: "屏蔽并封禁上传者" };
    if (!confirm(`确定${labels[action]}？`)) return;
    setBusy(true);
    try {
      await adminApi.resolveReport(r.id, action);
      await load();
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card>
      <div className="flex items-center gap-2 mb-4">
        {[
          { v: "open", l: "待处理" },
          { v: "resolved", l: "已处理" },
        ].map((o) => (
          <button
            key={o.v}
            onClick={() => setStatus(o.v)}
            className={`px-2.5 py-1 rounded-md text-xs transition ${
              status === o.v
                ? "bg-brand-600/20 text-brand-300 border border-brand-500/30"
                : "bg-neutral-800 text-neutral-400 hover:text-neutral-100 border border-transparent"
            }`}
          >
            {o.l}
          </button>
        ))}
      </div>

      {reports.length === 0 ? (
        <div className="py-10 text-center text-xs text-neutral-600">没有举报</div>
      ) : (
        <div className="space-y-2">
          {reports.map((r) => (
            <div key={r.id} className="rounded-xl border border-neutral-800 bg-neutral-950/40 p-3">
              {/* The whole summary is the affordance: a report is something you
                  read before acting on, so opening it should not require
                  hunting for a "详情" link. */}
              <button
                onClick={() => setDetail(r)}
                className="group flex w-full items-start gap-2.5 text-left"
              >
                <span className="mt-0.5 shrink-0 rounded-md bg-red-900/40 px-1.5 py-0.5 text-[10px] text-red-300">
                  {reportCategoryLabel(r.category)}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-xs text-neutral-200 group-hover:text-brand-300">
                    {r.reason || "（无补充说明）"}
                  </span>
                  <span className="mt-1 block text-[10px] text-neutral-600">
                    {r.orig_name} · 上传者 {r.owner_email} ·{" "}
                    {new Date(r.created_at).toLocaleString("zh-CN")}
                    {r.reports_on_image > 1 && (
                      <span className="ml-1 text-amber-300">· 该图共 {r.reports_on_image} 次举报</span>
                    )}
                  </span>
                </span>
                <i className="fa-solid fa-chevron-right mt-1 shrink-0 text-[9px] text-neutral-700 group-hover:text-brand-400" />
              </button>

              {status === "open" && (
                <div className="mt-2.5 flex items-center gap-1.5 border-t border-neutral-800/60 pt-2.5">
                  <button
                    onClick={() => resolve(r, "dismiss")}
                    disabled={busy}
                    className="inline-flex h-7 items-center justify-center rounded-md bg-neutral-800 px-3 text-[11px] text-neutral-300 transition hover:bg-neutral-700 disabled:opacity-50"
                  >
                    驳回
                  </button>
                  <button
                    onClick={() => resolve(r, "block")}
                    disabled={busy}
                    className="inline-flex h-7 items-center justify-center rounded-md bg-amber-700 px-3 text-[11px] text-white transition hover:bg-amber-600 disabled:opacity-50"
                  >
                    屏蔽
                  </button>
                  <button
                    onClick={() => resolve(r, "block_and_ban")}
                    disabled={busy}
                    className="inline-flex h-7 items-center justify-center rounded-md bg-red-600 px-3 text-[11px] text-white transition hover:bg-red-700 disabled:opacity-50"
                  >
                    屏蔽 + 封禁
                  </button>
                  <div className="flex-1" />
                  <button
                    onClick={() => setDetail(r)}
                    className="inline-flex h-7 items-center justify-center rounded-md px-2 text-[11px] text-neutral-500 transition hover:text-brand-300"
                  >
                    详情
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {detail && (
        <ReportDetail
          report={detail}
          onClose={() => setDetail(null)}
          onResolve={async (action) => {
            await resolve(detail, action);
            setDetail(null);
          }}
        />
      )}
    </Card>
  );
}

/**
 * Full report, with the image alongside it.
 *
 * Judging a report means looking at what was reported, and an admin who has to
 * copy a URL into another tab to do that will decide without looking. The
 * preview is behind one click rather than shown outright: the whole premise is
 * that the content might be something they did not ask to see.
 */
function ReportDetail({
  report,
  onClose,
  onResolve,
}: {
  report: Report;
  onClose: () => void;
  onResolve: (a: "dismiss" | "block" | "block_and_ban") => Promise<void>;
}) {
  const [revealed, setRevealed] = useState(false);
  const [busy, setBusy] = useState(false);

  async function act(a: "dismiss" | "block" | "block_and_ban") {
    setBusy(true);
    try {
      await onResolve(a);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center px-4">
      <div className="absolute inset-0 bg-black/50" onClick={() => !busy && onClose()} />
      <div className="shadow-panel relative flex max-h-[88vh] w-full max-w-lg flex-col overflow-hidden rounded-2xl border border-neutral-800 bg-neutral-900">
        <div className="flex items-center gap-2 border-b border-neutral-800/60 px-5 py-3.5">
          <span className="rounded-md bg-red-900/40 px-1.5 py-0.5 text-[10px] text-red-300">
            {reportCategoryLabel(report.category)}
          </span>
          <span className="text-sm text-neutral-100">举报详情</span>
          <div className="flex-1" />
          <button
            onClick={onClose}
            aria-label="关闭"
            className="inline-flex h-7 w-7 items-center justify-center rounded-lg text-neutral-500 transition hover:bg-neutral-800 hover:text-neutral-200"
          >
            <i className="fa-solid fa-xmark" />
          </button>
        </div>

        <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-5 py-4">
          <div>
            <div className="mb-1 text-[10px] text-neutral-500">举报说明</div>
            <div className="rounded-lg border border-neutral-800 bg-neutral-950/60 px-3 py-2.5 text-xs leading-relaxed whitespace-pre-wrap text-neutral-200">
              {report.reason || "（举报人未填写补充说明）"}
            </div>
          </div>

          <dl className="space-y-1.5 text-[11px]">
            <DetailRow label="图片" value={report.orig_name} />
            <DetailRow label="尺寸" value={`${report.width} × ${report.height} · ${formatBytes(report.size_stored)}`} />
            <DetailRow label="上传者" value={report.owner_email} />
            <DetailRow label="举报人" value={report.anonymous ? "匿名访客" : "已登录用户"} />
            <DetailRow label="联系方式" value={report.contact || "未留"} />
            <DetailRow label="举报时间" value={new Date(report.created_at).toLocaleString("zh-CN")} />
            <DetailRow
              label="该图举报次数"
              value={
                report.reports_on_image > 1 ? (
                  <span className="text-amber-300">{report.reports_on_image} 次</span>
                ) : (
                  "1 次"
                )
              }
            />
            <DetailRow label="图片状态" value={report.image_status === "blocked" ? "已屏蔽" : "正常"} />
          </dl>

          <div>
            <div className="mb-1 text-[10px] text-neutral-500">对象路径</div>
            <div className="rounded-lg border border-neutral-800 bg-neutral-950/60 px-3 py-2 font-mono text-[10px] break-all text-neutral-400">
              {report.object_key}
            </div>
          </div>

          <div>
            <div className="mb-1.5 flex items-center gap-2">
              <span className="text-[10px] text-neutral-500">图片预览</span>
              {report.short_code && (
                <a
                  href={`/${report.short_code}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-[10px] text-brand-400 hover:underline"
                >
                  打开分享页 →
                </a>
              )}
            </div>
            {revealed ? (
              <img
                src={`/storage/${report.object_key}`}
                alt=""
                className="max-h-64 w-full rounded-lg border border-neutral-800 object-contain"
              />
            ) : (
              <button
                onClick={() => setRevealed(true)}
                className="flex h-32 w-full items-center justify-center rounded-lg border border-dashed border-neutral-700 text-xs text-neutral-500 transition hover:border-brand-500 hover:text-brand-300"
              >
                <i className="fa-solid fa-eye mr-1.5" />
                点击查看被举报的图片
              </button>
            )}
          </div>
        </div>

        <div className="flex items-center gap-1.5 border-t border-neutral-800/60 px-5 py-3">
          <button
            onClick={() => act("dismiss")}
            disabled={busy}
            className="inline-flex h-8 items-center justify-center rounded-lg bg-neutral-800 px-3 text-xs text-neutral-300 transition hover:bg-neutral-700 disabled:opacity-50"
          >
            驳回
          </button>
          <button
            onClick={() => act("block")}
            disabled={busy}
            className="inline-flex h-8 items-center justify-center rounded-lg bg-amber-700 px-3 text-xs text-white transition hover:bg-amber-600 disabled:opacity-50"
          >
            屏蔽
          </button>
          <button
            onClick={() => act("block_and_ban")}
            disabled={busy}
            className="inline-flex h-8 items-center justify-center rounded-lg bg-red-600 px-3 text-xs font-medium text-white transition hover:bg-red-700 disabled:opacity-50"
          >
            屏蔽 + 封禁
          </button>
          <div className="flex-1" />
          <button
            onClick={onClose}
            disabled={busy}
            className="inline-flex h-8 items-center justify-center rounded-lg px-3 text-xs text-neutral-400 transition hover:text-neutral-100"
          >
            关闭
          </button>
        </div>
      </div>
    </div>
  );
}

function DetailRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-baseline justify-between gap-3">
      <dt className="shrink-0 text-neutral-600">{label}</dt>
      <dd className="min-w-0 truncate text-neutral-300">{value}</dd>
    </div>
  );
}

/* ---------- Ledger ---------- */

function LedgerTab() {
  const [txs, setTxs] = useState<AdminQuotaTx[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [perPage, setPerPage] = useState(50);
  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async (p: number, q: string, n: number) => {
    setBusy(true);
    try {
      const r = await adminApi.listTransactions({ limit: n, offset: p * n, q: q || undefined });
      setTxs(r.transactions);
      setTotal(r.total);
    } catch {
      // Leave the previous page on screen rather than blanking the table: a
      // transient failure should not look like "there are no records".
    } finally {
      setBusy(false);
    }
  }, []);

  // Debounced, matching the gallery's 300ms. The empty case fires immediately
  // so clearing the box feels instant.
  //
  // page is deliberately not in the dependency list — paging calls load()
  // directly. Having both would fire two requests for one click.
  useEffect(() => {
    const timer = setTimeout(() => {
      setPage(0);
      load(0, query, perPage);
    }, query ? 300 : 0);
    return () => clearTimeout(timer);
  }, [query, perPage, load]);

  const cols = ["用户", "文件", "类型", "变化", "配额后", "已用后", "说明", "时间"];

  return (
    <Card>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <div className="relative flex-1 min-w-[14rem]">
          <i className="fa-solid fa-magnifying-glass absolute left-3 top-1/2 -translate-y-1/2 text-[10px] text-neutral-600" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="搜索邮箱、昵称、文件名或说明"
            maxLength={128}
            className="h-8 w-full rounded-lg border border-neutral-800 bg-neutral-950 pl-8 pr-8 text-xs text-neutral-200 outline-none placeholder:text-neutral-600 focus:border-neutral-700"
          />
          {query && (
            <button
              onClick={() => setQuery("")}
              title="清空"
              className="absolute right-2 top-1/2 -translate-y-1/2 text-[10px] text-neutral-600 hover:text-neutral-300"
            >
              <i className="fa-solid fa-xmark" />
            </button>
          )}
        </div>
        <span className="text-[10px] text-neutral-600 tabular-nums">共 {total} 条</span>
        <PageSizeMenu value={perPage} onChange={setPerPage} />
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead className="text-[10px] text-neutral-600">
            <tr>
              {cols.map((h) => (
                <th key={h} className="pb-2 font-normal whitespace-nowrap pr-3">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-neutral-800/50">
            {txs.map((t) => (
              <tr key={t.id} className="text-neutral-400">
                <td className="py-2 pr-3 whitespace-nowrap" title={t.user_name || undefined}>
                  {t.user_email}
                </td>
                <td className="py-2 pr-3 max-w-[12rem] truncate" title={t.image_name ?? undefined}>
                  {t.image_name ?? <span className="text-neutral-700">—</span>}
                </td>
                <td className="py-2 pr-3 whitespace-nowrap">{t.type}</td>
                <td
                  className={`py-2 pr-3 whitespace-nowrap ${
                    t.bytes >= 0 ? "text-teal-400" : "text-neutral-400"
                  }`}
                >
                  {t.bytes >= 0 ? "+" : "−"}
                  {formatBytes(Math.abs(t.bytes))}
                </td>
                <td className="py-2 pr-3 whitespace-nowrap">{formatBytes(t.quota_after, 0)}</td>
                <td className="py-2 pr-3 whitespace-nowrap">{formatBytes(t.used_after, 0)}</td>
                <td className="py-2 pr-3 max-w-[16rem] truncate" title={t.reason}>
                  {t.reason}
                </td>
                <td className="py-2 whitespace-nowrap text-[10px]">
                  {new Date(t.created_at).toLocaleString("zh-CN", { hour12: false })}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {txs.length === 0 && !busy && (
          <div className="py-10 text-center text-xs text-neutral-600">
            {query ? "没有匹配的记录" : "暂无记录"}
          </div>
        )}
      </div>

      {total > perPage && (
        <Pager
          page={page}
          perPage={perPage}
          total={total}
          busy={busy}
          onPage={(p) => {
            setPage(p);
            load(p, query, perPage);
          }}
        />
      )}
    </Card>
  );
}

function Card({ children }: { children: React.ReactNode }) {
  return <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-4">{children}</div>;
}
