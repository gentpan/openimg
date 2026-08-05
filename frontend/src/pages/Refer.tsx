import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../AuthContext";
import { formatBytes, referralApi } from "../api";
import Footer from "../Footer";
import Nav from "../components/Nav";

export default function ReferPage() {
  const { user } = useAuth();
  const [s, setS] = useState<{
    code: string;
    count: number;
    total_bytes: number;
    bonus_bytes: number;
  } | null>(null);
  const [invitees, setInvitees] = useState<{ name: string; email: string; created_at: string }[]>([]);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!user) return;
    referralApi.me().then(setS).catch(() => {});
    referralApi.list().then(setInvitees).catch(() => {});
  }, [user]);

  if (!user) {
    return (
      <div className="min-h-screen flex items-center justify-center text-neutral-500">
        请先 <Link to="/login" className="text-brand-400 hover:underline mx-1">登录</Link>
      </div>
    );
  }

  const link = s?.code ? `${window.location.origin}/?ref=${s.code}` : "";
  const bonus = formatBytes(s?.bonus_bytes ?? 0, 0);

  async function copy() {
    if (!link) return;
    try {
      await navigator.clipboard.writeText(link);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch (e) {
      alert("复制失败：" + e);
    }
  }

  return (
    <div className="min-h-screen flex flex-col bg-neutral-950">
      <Nav />
      <div className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 py-8">
        <h1 className="text-lg font-brand text-neutral-100 mb-5">邀请有奖</h1>

        {/* Hero */}
        <div className="rounded-2xl bg-gradient-to-br from-brand-900/40 via-brand-800/20 to-fuchsia-900/30 border border-brand-900/40 p-6 sm:p-8 mb-8 flex flex-col sm:flex-row sm:items-center gap-6">
          <div className="flex-1">
            <h2 className="text-2xl sm:text-3xl font-brand leading-tight mb-2">
              邀请好友，双方各得 <span className="text-accent-300">{bonus} 空间</span>
            </h2>
            <p className="text-sm text-neutral-300/80 mb-5">把链接分享给朋友，他们注册成功后，你和他都立即到账存储空间。</p>
            <div className="flex items-stretch gap-2 max-w-xl">
              <input
                readOnly
                value={link}
                onFocus={(e) => e.currentTarget.select()}
                className="flex-1 rounded-lg bg-neutral-950/70 border border-neutral-800 px-3 py-2 text-sm font-mono text-neutral-200"
              />
              <button
                onClick={copy}
                className="rounded-lg bg-brand-600 hover:bg-brand-500 px-4 py-2 text-sm font-medium text-white inline-flex items-center gap-1.5"
              >
                <i className={`fa-solid ${copied ? "fa-check" : "fa-copy"}`} />
                {copied ? "已复制" : "复制"}
              </button>
            </div>
            {s?.code && (
              <div className="mt-3 text-xs text-neutral-500">
                你的推荐码 <span className="font-mono text-accent-300">{s.code}</span>
              </div>
            )}
          </div>
          <div className="hidden sm:block text-7xl text-accent-300/60">
            <i className="fa-solid fa-gift" />
          </div>
        </div>

        {/* Two-column "they get / you get" */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-8">
          <Card title="他们获得">
            <Bullet>
              一次性 <b className="text-accent-200">{bonus}</b> — 通过你的链接注册即到账
            </Bullet>
            <Bullet>立即解锁所有免费用户额度</Bullet>
          </Card>
          <Card title="你获得">
            <Bullet>
              一次性 <b className="text-accent-200">{bonus}</b> — 每位有效注册的好友
            </Bullet>
            <Bullet>邀请越多，空间越多（受用户组上限约束）</Bullet>
          </Card>
        </div>

        {/* Summary */}
        <h3 className="text-sm uppercase tracking-wide text-neutral-500 mb-3">数据</h3>
        <div className="grid grid-cols-2 md:grid-cols-3 gap-3 mb-8">
          <Stat icon="fa-user-plus" iconBg="bg-teal-900/40 text-teal-300" label="已邀请用户" value={s?.count ?? 0} />
          <Stat icon="fa-database" iconBg="bg-accent-900/40 text-accent-300" label="累计获得空间" value={formatBytes(s?.total_bytes ?? 0, 0)} />
          <Stat icon="fa-gift" iconBg="bg-amber-900/40 text-amber-300" label="单次奖励" value={`${bonus} × 2`} />
        </div>

        {/* Invitees table */}
        <h3 className="text-sm uppercase tracking-wide text-neutral-500 mb-3">邀请记录</h3>
        <div className="overflow-x-auto rounded-xl border border-neutral-800">
          <table className="w-full text-sm">
            <thead className="bg-neutral-900/60 text-xs uppercase text-neutral-500">
              <tr>
                <th className="text-left px-3 py-2 font-normal">用户</th>
                <th className="text-left px-3 py-2 font-normal">邮箱</th>
                <th className="text-left px-3 py-2 font-normal">注册时间</th>
              </tr>
            </thead>
            <tbody>
              {invitees.length === 0 ? (
                <tr>
                  <td colSpan={3} className="text-center text-neutral-500 py-8 text-xs">
                    还没有人通过你的链接注册，去复制链接分享给朋友吧
                  </td>
                </tr>
              ) : (
                invitees.map((u, i) => (
                  <tr key={i} className="border-t border-neutral-900">
                    <td className="px-3 py-2">{u.name || "—"}</td>
                    <td className="px-3 py-2 font-mono text-xs text-neutral-400">{u.email}</td>
                    <td className="px-3 py-2 text-xs text-neutral-500 whitespace-nowrap">{u.created_at}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
      <Footer />
    </div>
  );
}

function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-neutral-800 bg-neutral-900/40 p-5">
      <h3 className="text-base font-medium mb-3">{title}</h3>
      <ul className="space-y-2">{children}</ul>
    </div>
  );
}

function Bullet({ children }: { children: React.ReactNode }) {
  return (
    <li className="flex items-start gap-2 text-sm text-neutral-300">
      <i className="fa-solid fa-circle-check text-teal-500 mt-0.5" />
      <span>{children}</span>
    </li>
  );
}

function Stat({ icon, iconBg, label, value }: { icon: string; iconBg: string; label: string; value: number | string }) {
  return (
    <div className="rounded-xl border border-neutral-800 bg-neutral-900/40 p-4 flex items-center gap-3">
      <div className={`h-10 w-10 rounded-lg flex items-center justify-center ${iconBg}`}>
        <i className={`fa-solid ${icon}`} />
      </div>
      <div>
        <div className="text-2xl font-semibold leading-none">{value}</div>
        <div className="text-xs text-neutral-500 mt-1">{label}</div>
      </div>
    </div>
  );
}
