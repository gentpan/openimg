import { useState } from "react";
import { Link, useLocation } from "react-router-dom";
import { useAuth } from "../AuthContext";
import Logo from "../Logo";
import { formatBytes, quotaApi } from "../api";
import { RingSpinner } from "./Spinner";
import { useBrand } from "../BrandContext";
import { useLang } from "../LangContext";
import { useToast } from "../ToastContext";
import Avatar from "./Avatar";

/**
 * Shared top navigation. Every page had its own copy before; the space pill is
 * the one element that must stay identical everywhere, since it's how a user
 * knows whether their next upload will fit.
 */
export default function Nav() {
  const { user, logout, refresh } = useAuth();
  const { brand, setBrand } = useBrand();
  const { lang, setLang } = useLang();
  const toast = useToast();
  const { pathname } = useLocation();
  const [busy, setBusy] = useState(false);

  const pct = user && user.quota_bytes > 0 ? Math.min(100, (user.used_bytes / user.quota_bytes) * 100) : 0;
  const low = pct >= 90;

  // Check-in lives in the header because it's a daily habit, not a destination:
  // making people navigate to a page to click one button is how a streak dies.
  async function checkin() {
    setBusy(true);
    try {
      const res = await quotaApi.checkin();
      if (res.capped) {
        toast.info("空间已达上限", "今天的签到没有增加容量");
      } else {
        toast.success(`签到成功，+${formatBytes(res.granted_bytes)}`, "已永久累加到你的总容量");
      }
      await refresh();
    } catch (e) {
      toast.error("签到失败", e instanceof Error ? e.message : undefined);
    } finally {
      setBusy(false);
    }
  }

  return (
    <nav className="sticky top-0 z-30 border-b border-neutral-800/60 bg-neutral-950/90 backdrop-blur-xl">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 h-14 flex items-center justify-between gap-3">
        {/* Full document load rather than a client-side route change — the
            brand doubles as a "start over" control, and a route change keeps
            every stale bit of state it is meant to clear. */}
        <a href="/" className="flex items-center gap-2.5 shrink-0">
          <Logo size={24} asLink={false} />
          <span className="text-base font-brand">
            Open<span className="text-brand-400">img</span>
          </span>
        </a>

        <div className="flex items-center gap-2.5 text-xs">
          {user && (
            <>
              <NavItem to="/dashboard" icon="fa-gauge" label="概览" active={pathname === "/dashboard"} />
              <NavItem to="/upload" icon="fa-cloud-arrow-up" label="上传" active={pathname === "/upload"} />
              <NavItem to="/gallery" icon="fa-images" label="图库" active={pathname === "/gallery"} />
              <NavItem to="/refer" icon="fa-gift" label="邀请" active={pathname === "/refer"} />
            </>
          )}
          {user?.role === "admin" && (
            <Link to="/admin" className="text-brand-400 hover:underline hidden sm:inline">
              管理
            </Link>
          )}
          {user ? (
            <>
              <button
                onClick={checkin}
                disabled={busy || user.checked_in_today}
                title={
                  user.checked_in_today
                    ? `今日已签到 · 连续 ${user.checkin_streak} 天`
                    : "点击签到，随机领取空间"
                }
                className={`rounded-full px-2.5 py-0.5 transition whitespace-nowrap ${
                  user.checked_in_today
                    ? "bg-neutral-900 text-neutral-600 cursor-default"
                    : "bg-brand-600 text-brand-ink hover:bg-brand-500"
                }`}
              >
                {busy ? (
                  <RingSpinner className="h-3.5 w-3.5 inline-block align-[-2px]" />
                ) : user.checked_in_today ? (
                  <>
                    <i className="fa-solid fa-check text-[10px] mr-1" />
                    {user.checkin_streak} 天
                  </>
                ) : (
                  <>
                    <i className="fa-solid fa-calendar-check text-[10px] mr-1" />
                    签到
                  </>
                )}
              </button>
              <Link
                to="/space"
                title={`已用 ${formatBytes(user.used_bytes)} / 共 ${formatBytes(user.quota_bytes)}`}
                className={`rounded-full px-2 py-0.5 transition ${
                  low
                    ? "bg-amber-900/30 text-amber-200 hover:bg-amber-900/50"
                    : "bg-brand-900/30 text-brand-200 hover:bg-brand-900/50"
                }`}
              >
                <i className="fa-solid fa-database text-[10px] mr-1" />
                {formatBytes(user.available_bytes, 0)}
              </Link>
              <button
                onClick={() => setBrand(brand === "green" ? "violet" : "green")}
                title={brand === "green" ? "切换到紫色" : "切换到绿色"}
                aria-label={brand === "green" ? "切换到紫色" : "切换到绿色"}
                className="w-6 h-6 rounded-full text-neutral-500 hover:text-brand-300 hover:bg-neutral-900 transition inline-flex items-center justify-center"
              >
                {/* The swatch is the brand colour itself — a paint-can icon
                    would say "theme" without saying which one is on. */}
                <span className="w-3 h-3 rounded-full bg-brand-600 ring-1 ring-white/25" />
              </button>
              <button
                onClick={() => setLang(lang === "zh" ? "en" : "zh")}
                title={lang === "zh" ? "Switch to English" : "切换到中文"}
                aria-label={lang === "zh" ? "Switch to English" : "切换到中文"}
                className="h-6 rounded-full px-1.5 text-[10px] font-medium text-neutral-500 hover:text-brand-300 hover:bg-neutral-900 transition"
              >
                {/* The label is the language you get, not the one you are in —
                    a switch that shows its current state reads as a status
                    light and gets ignored. */}
                {lang === "zh" ? "EN" : "中"}
              </button>
              <Link
                to="/settings"
                className="flex items-center gap-1.5 text-neutral-400 hover:text-brand-300 min-w-0"
              >
                <Avatar user={user} size={20} />
                <span className="truncate max-w-[8rem]">{user.name || user.email}</span>
              </Link>
              <button onClick={() => logout()} className="text-neutral-600 hover:text-neutral-300">
                退出
              </button>
            </>
          ) : (
            <>
              <button
                onClick={() => setBrand(brand === "green" ? "violet" : "green")}
                title={brand === "green" ? "切换到紫色" : "切换到绿色"}
                aria-label={brand === "green" ? "切换到紫色" : "切换到绿色"}
                className="w-6 h-6 rounded-full text-neutral-500 hover:text-brand-300 hover:bg-neutral-900 transition inline-flex items-center justify-center"
              >
                {/* The swatch is the brand colour itself — a paint-can icon
                    would say "theme" without saying which one is on. */}
                <span className="w-3 h-3 rounded-full bg-brand-600 ring-1 ring-white/25" />
              </button>
              <Link to="/login" className="text-brand-400 hover:underline">
                登录
              </Link>
              <Link to="/register" className="text-neutral-500 hover:text-neutral-200">
                注册
              </Link>
            </>
          )}
        </div>
      </div>
    </nav>
  );
}

function NavItem({ to, icon, label, active }: { to: string; icon: string; label: string; active: boolean }) {
  return (
    <Link
      to={to}
      title={label}
      className={`transition ${active ? "text-brand-300" : "text-neutral-400 hover:text-brand-300"}`}
    >
      <i className={`fa-solid ${icon}`} />
      <span className="ml-1.5 hidden md:inline">{label}</span>
    </Link>
  );
}
