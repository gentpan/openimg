import Icon from "../Icon";
import { useLayoutEffect, useRef, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import { useAuth } from "../AuthContext";
import Logo from "../Logo";
import { formatBytes, quotaApi } from "../api";
import { refreshAIStatus, useAIStatus } from "../aiStatus";
import { RingSpinner } from "./Spinner";
import { useBrand } from "../BrandContext";
import { useLang } from "../LangContext";
import { useToast } from "../ToastContext";
import Avatar from "./Avatar";
import GroupBadge from "./GroupBadge";
import { prefetch } from "../prefetch";

/**
 * Shared top navigation. Every page had its own copy before; the space pill is
 * the one element that must stay identical everywhere, since it's how a user
 * knows whether their next upload will fit.
 */
export default function Nav() {
  const { t } = useLang();
  const railRef = useRef<HTMLDivElement>(null);
  const { user, logout, refresh } = useAuth();
  const { brand, setBrand } = useBrand();
  const toast = useToast();
  const { pathname } = useLocation();
  const [busy, setBusy] = useState(false);
  // Signed-out visitors never ask: the endpoint needs a session, and the entry
  // it controls is inside the signed-in half of the bar anyway.
  const { status: ai } = useAIStatus(!!user);
  const aiEnabled = ai?.enabled === true;

  // 发光条停在哪、有多宽,是量出来的。
  //
  // pic.bi 那边能用 calc(100% / 数量) 算,是因为它只有两个等宽标签。这里最多
  // 六个、文案长短不一,而且 md 以下整排标签文字会隐藏、宽度当场腰斩——任何
  // 算出来的分数都会错位。
  //
  // useLayoutEffect 而不是 useEffect:布局阶段就写好变量,浏览器绘制第一帧时
  // 位置已经是对的,否则换页会看见它从旧位置闪一下。
  useLayoutEffect(() => {
    const rail = railRef.current;
    if (!rail) return;

    const place = () => {
      const on = rail.querySelector<HTMLElement>(".nav-tab.is-active");
      // 停留在原地而不是缩成 0:当前页不在导航里(比如设置页)时,让它留在上
      // 一格比让它凭空消失更少突兀。
      if (!on) return;
      rail.style.setProperty("--nav-x", `${on.offsetLeft}px`);
      rail.style.setProperty("--nav-w", `${on.offsetWidth}px`);
    };
    place();

    // 宽度会在两种情况下变而 pathname 不变:窗口跨过 md 断点(标签文字显隐)、
    // 以及字体加载完成后文字重新排版。ResizeObserver 两种都盯得住。
    const ro = new ResizeObserver(place);
    ro.observe(rail);
    for (const el of rail.querySelectorAll(".nav-tab")) ro.observe(el);
    return () => ro.disconnect();
  }, [pathname, user, aiEnabled, t]);

  const pct = user && user.quota_bytes > 0 ? Math.min(100, (user.used_bytes / user.quota_bytes) * 100) : 0;
  const low = pct >= 90;

  // Check-in lives in the header because it's a daily habit, not a destination:
  // making people navigate to a page to click one button is how a streak dies.
  async function checkin() {
    setBusy(true);
    try {
      const res = await quotaApi.checkin();
      // Check-in also throws in AI generations, and that is the advertised way
      // out of a spent monthly allowance — so say it happened, and resync the
      // count the generate page reads.
      const ai = res.ai_credits ?? 0;
      if (res.capped) {
        toast.info(t.nav.checkinCappedTitle, t.nav.checkinCappedDetail);
      } else {
        toast.success(
          t.nav.checkinSuccessTitle(formatBytes(res.granted_bytes)),
          ai > 0
            ? `${t.nav.checkinSuccessDetail} · ${t.generate.quota.checkinGranted(ai)}`
            : t.nav.checkinSuccessDetail,
        );
      }
      await Promise.all([refresh(), refreshAIStatus()]);
    } catch (e) {
      toast.error(t.nav.checkinFailed, e instanceof Error ? e.message : undefined);
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

        <div className="flex items-center gap-2.5 text-xs self-stretch">
          {user && (
            <div ref={railRef} className="nav-rail">
              <NavItem to="/dashboard" icon="gauge" label={t.nav.overview} active={pathname === "/dashboard"} />
              <NavItem to="/upload" icon="cloud-arrow-up" label={t.common.upload} active={pathname === "/upload"} />
              <NavItem to="/gallery" icon="images" label={t.nav.gallery} active={pathname === "/gallery"} />
              {/* Absent, not disabled, where the deployment configured no AI
                  key: there is nothing behind the link on such a build. */}
              {aiEnabled && (
                <>
                  <NavItem
                    to="/generate"
                    icon="wand-magic-sparkles"
                    label={t.nav.generate}
                    active={pathname === "/generate"}
                  />
                  <NavItem
                    to="/retouch"
                    icon="wand-magic"
                    label={t.nav.retouch}
                    active={pathname === "/retouch"}
                  />
                </>
              )}
              <NavItem to="/refer" icon="gift" label={t.nav.refer} active={pathname === "/refer"} />
              <span className="nav-glider" aria-hidden="true" />
            </div>
          )}
          {user?.role === "admin" && (
            <Link to="/admin" className="text-brand-400 hover:underline hidden sm:inline">
              {t.nav.admin}
            </Link>
          )}
          {/* 下载入口。放在登录/未登录两个分支之外——两种状态都该看得到,
              而写在分支里就得维护两份。小屏藏起来:那里已经很挤,而首页本来
              就有一整块讲这件事。 */}
          <Link
            to="/download"
            onMouseEnter={() => prefetch("/download")}
            onFocus={() => prefetch("/download")}
            className={`hidden sm:inline-flex items-center gap-1 whitespace-nowrap transition ${
              pathname === "/download"
                ? "text-brand-300"
                : "text-neutral-500 hover:text-brand-300"
            }`}
          >
            <Icon name="download" className="text-[10px]" />
            {t.nav.download}
          </Link>
          {user ? (
            <>
              <button
                onClick={checkin}
                disabled={busy || user.checked_in_today}
                title={
                  user.checked_in_today
                    ? t.nav.checkedInTodayStreak(user.checkin_streak)
                    : t.nav.checkinHint
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
                    <Icon name="check" className="text-[10px] mr-1"  />
                    {t.common.days(user.checkin_streak)}
                  </>
                ) : (
                  <>
                    <Icon name="calendar-check" className="text-[10px] mr-1"  />
                    {t.nav.checkin}
                  </>
                )}
              </button>
              <Link
                to="/space"
                title={t.nav.storagePillTitle(formatBytes(user.used_bytes), formatBytes(user.quota_bytes))}
                onMouseEnter={() => prefetch("/space")}
                onFocus={() => prefetch("/space")}
                className={`rounded-full px-2 py-0.5 transition ${
                  low
                    ? "bg-amber-900/30 text-amber-200 hover:bg-amber-900/50"
                    : "bg-brand-900/30 text-brand-200 hover:bg-brand-900/50"
                }`}
              >
                <Icon name="database" className="text-[10px] mr-1"  />
                {formatBytes(user.available_bytes, 0)}
              </Link>
              <button
                onClick={() => setBrand(brand === "green" ? "violet" : "green")}
                title={brand === "green" ? t.nav.switchToViolet : t.nav.switchToGreen}
                aria-label={brand === "green" ? t.nav.switchToViolet : t.nav.switchToGreen}
                className="w-6 h-6 rounded-full text-neutral-500 hover:text-brand-300 hover:bg-neutral-900 transition inline-flex items-center justify-center"
              >
                {/* The swatch is the brand colour itself — a paint-can icon
                    would say "theme" without saying which one is on. */}
                <span className="w-3 h-3 rounded-full bg-brand-600 ring-1 ring-white/25" />
              </button>
              <LangFlagButton />
              <Link
                to="/settings"
                onMouseEnter={() => prefetch("/settings")}
                onFocus={() => prefetch("/settings")}
                className="flex items-center gap-1.5 text-neutral-400 hover:text-brand-300 min-w-0"
              >
                <Avatar user={user} size={20} />
                <span className="truncate max-w-[8rem]">{user.name || user.email}</span>
              </Link>
              {/* Hidden on small screens: the nav is already tight there, and
                  the tier is visible on the pages themselves. */}
              {user.group && (
                <span className="hidden md:inline-flex">
                  <GroupBadge name={user.group} />
                </span>
              )}
              {/* Icon-only, but never silent: title gives sighted users the
                  hover hint and aria-label keeps it named for screen readers. */}
              <button
                onClick={() => logout()}
                title={t.common.signOut}
                aria-label={t.common.signOut}
                className="text-neutral-600 hover:text-red-400 transition"
              >
                <Icon name="right-from-bracket"  />
              </button>
            </>
          ) : (
            <>
              <button
                onClick={() => setBrand(brand === "green" ? "violet" : "green")}
                title={brand === "green" ? t.nav.switchToViolet : t.nav.switchToGreen}
                aria-label={brand === "green" ? t.nav.switchToViolet : t.nav.switchToGreen}
                className="w-6 h-6 rounded-full text-neutral-500 hover:text-brand-300 hover:bg-neutral-900 transition inline-flex items-center justify-center"
              >
                {/* The swatch is the brand colour itself — a paint-can icon
                    would say "theme" without saying which one is on. */}
                <span className="w-3 h-3 rounded-full bg-brand-600 ring-1 ring-white/25" />
              </button>
              <LangFlagButton />
              {/* One entry point: the login page carries the "no account yet →
                  sign up" prompt, so a separate register link only splits the
                  click. Icon mirrors the logout door icon, direction flipped. */}
              <Link
                to="/login"
                className="rounded-full px-3 py-0.5 bg-brand-600 text-brand-ink hover:bg-brand-500 transition whitespace-nowrap"
                onMouseEnter={() => prefetch("/login")}
                onFocus={() => prefetch("/login")}
              >
                <Icon name="right-to-bracket" className="text-[10px] mr-1"  />
                {t.common.signIn}
              </Link>
            </>
          )}
        </div>
      </div>
    </nav>
  );
}

function LangFlagButton() {
  const { lang, setLang } = useLang();
  return (
    <button
      onClick={() => setLang(lang === "zh" ? "en" : "zh")}
      title={lang === "zh" ? "Switch to English" : "切换到中文"}
      aria-label={lang === "zh" ? "Switch to English" : "切换到中文"}
      className="w-6 h-6 rounded-full hover:bg-neutral-900 transition inline-flex items-center justify-center"
    >
      {/* The flag is the language you get, not the one you are in —
          a switch that shows its current state reads as a status
          light and gets ignored. */}
      <img
        src={lang === "zh" ? "/static/flags/us.svg" : "/static/flags/cn.svg"}
        alt=""
        className="w-5 h-5 rounded-full"
      />
    </button>
  );
}

function NavItem({ to, icon, label, active }: { to: string; icon: string; label: string; active: boolean }) {
  return (
    <Link
      to={to}
      title={label}
      aria-current={active ? "page" : undefined}
      className={`nav-tab${active ? " is-active" : ""}`}
      onMouseEnter={() => prefetch(to)}
      onFocus={() => prefetch(to)}
    >
      <Icon name={icon}  />
      <span className="hidden md:inline">{label}</span>
    </Link>
  );
}
