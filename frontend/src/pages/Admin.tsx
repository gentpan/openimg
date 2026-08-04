import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import Nav from "../components/Nav";
import Footer from "../Footer";
import AdminDashboard from "./admin/Dashboard";
import LoginMethods from "./admin/LoginMethods";
import UploadSettings from "./admin/UploadSettings";
import { adminApi, formatBytes } from "../api";
import type { AdminQuotaTx, AdminUser, Image, Report, UserGroup } from "../types";

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
          <h1 className="text-lg font-brand text-neutral-100">管理后台</h1>
          <div className="flex-1" />
          <Link to="/" className="text-xs text-neutral-500 hover:text-violet-300">
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
                  ? "bg-violet-600 text-white"
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
      {msg && <div className="mb-3 text-xs text-violet-300">{msg}</div>}
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
                  {u.role === "admin" && <span className="text-[9px] text-violet-400">管理员</span>}
                </td>
                <td className="py-2 pr-3">
                  <select
                    value={u.group_id || ""}
                    disabled={busy}
                    onChange={(e) => patch(u.id, { group_id: e.target.value })}
                    className="rounded-md bg-neutral-800 px-2 py-1 text-[11px] outline-none border border-transparent focus:border-violet-500"
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
                  <span className={u.status === "active" ? "text-emerald-400" : "text-red-400"}>
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
                  <button onClick={() => grant(u)} disabled={busy} className="text-violet-400 hover:underline mr-2">
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
                      className="text-emerald-400 hover:underline"
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
                    className="w-24 rounded-md bg-neutral-800 px-2 py-1 text-[11px] text-right outline-none border border-transparent focus:border-violet-500"
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
                className="accent-violet-600"
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
                className="w-40 rounded-md bg-neutral-800 px-2 py-1 text-[11px] outline-none border border-transparent focus:border-violet-500"
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
  const [images, setImages] = useState<Image[]>([]);
  const [status, setStatus] = useState("");

  async function load() {
    setImages(await adminApi.listImages({ status: status || undefined, limit: 60 }));
  }
  useEffect(() => {
    load().catch(() => {});
  }, [status]);

  async function toggleBlock(img: Image) {
    await adminApi.blockImage(img.id);
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
                ? "bg-violet-600/20 text-violet-300 border border-violet-500/30"
                : "bg-neutral-800 text-neutral-400 hover:text-neutral-100 border border-transparent"
            }`}
          >
            {o.l}
          </button>
        ))}
      </div>
      <div className="grid grid-cols-3 sm:grid-cols-5 lg:grid-cols-8 gap-2">
        {images.map((img) => (
          <div key={img.id} className="group relative rounded-xl overflow-hidden border border-neutral-800">
            <img src={img.thumb_url} alt="" loading="lazy" className="w-full aspect-square object-cover" />
            {img.status === "blocked" && (
              <div className="absolute inset-0 bg-red-950/60 flex items-center justify-center">
                <i className="fa-solid fa-eye-slash text-red-300" />
              </div>
            )}
            <button
              onClick={() => toggleBlock(img)}
              className="absolute inset-x-0 bottom-0 py-1 text-[10px] scrim opacity-0 group-hover:opacity-100 transition"
            >
              {img.status === "blocked" ? "解除屏蔽" : "屏蔽"}
            </button>
          </div>
        ))}
      </div>
      {images.length === 0 && <div className="py-10 text-center text-xs text-neutral-600">暂无图片</div>}
    </Card>
  );
}

/* ---------- Reports ---------- */

function ReportsTab({ onChange }: { onChange: (n: number) => void }) {
  const [reports, setReports] = useState<Report[]>([]);
  const [status, setStatus] = useState("open");
  const [busy, setBusy] = useState(false);

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
                ? "bg-violet-600/20 text-violet-300 border border-violet-500/30"
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
              <div className="flex items-start gap-3">
                <div className="flex-1 min-w-0">
                  <div className="text-xs text-neutral-200 break-words">{r.reason}</div>
                  <div className="mt-1 text-[10px] text-neutral-600">
                    上传者 {r.owner_email} · {new Date(r.created_at).toLocaleString("zh-CN")}
                    {r.contact && ` · 联系方式 ${r.contact}`}
                  </div>
                  <div className="mt-1 text-[10px] text-faint font-mono break-all">{r.object_key}</div>
                </div>
                {status === "open" && (
                  <div className="flex flex-col gap-1 shrink-0">
                    <button
                      onClick={() => resolve(r, "dismiss")}
                      disabled={busy}
                      className="rounded-md bg-neutral-800 px-2.5 py-1 text-[10px] text-neutral-300 hover:bg-neutral-700 transition"
                    >
                      驳回
                    </button>
                    <button
                      onClick={() => resolve(r, "block")}
                      disabled={busy}
                      className="rounded-md bg-amber-700 px-2.5 py-1 text-[10px] text-white hover:bg-amber-600 transition"
                    >
                      屏蔽
                    </button>
                    <button
                      onClick={() => resolve(r, "block_and_ban")}
                      disabled={busy}
                      className="rounded-md bg-red-600 px-2.5 py-1 text-[10px] text-white hover:bg-red-700 transition"
                    >
                      屏蔽+封禁
                    </button>
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

/* ---------- Ledger ---------- */

function LedgerTab() {
  const [txs, setTxs] = useState<AdminQuotaTx[]>([]);
  useEffect(() => {
    adminApi
      .listTransactions(200)
      .then(setTxs)
      .catch(() => {});
  }, []);

  return (
    <Card>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead className="text-[10px] text-neutral-600">
            <tr>
              {["用户", "类型", "变化", "配额后", "已用后", "说明", "时间"].map((h) => (
                <th key={h} className="pb-2 font-normal whitespace-nowrap pr-3">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-neutral-800/50">
            {txs.map((t) => (
              <tr key={t.id} className="text-neutral-400">
                <td className="py-2 pr-3 whitespace-nowrap">{t.user_email}</td>
                <td className="py-2 pr-3 whitespace-nowrap">{t.type}</td>
                <td
                  className={`py-2 pr-3 whitespace-nowrap ${
                    t.bytes >= 0 ? "text-emerald-400" : "text-neutral-400"
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
        {txs.length === 0 && <div className="py-10 text-center text-xs text-neutral-600">暂无记录</div>}
      </div>
    </Card>
  );
}

function Card({ children }: { children: React.ReactNode }) {
  return <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-4">{children}</div>;
}
