import { useState } from "react";
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

/**
 * Shared top navigation. Every page had its own copy before; the space pill is
 * the one element that must stay identical everywhere, since it's how a user
 * knows whether their next upload will fit.
 */
export default function Nav() {
  const { t } = useLang();
  const { user, logout, refresh } = useAuth();
  const { brand, setBrand } = useBrand();
  const toast = useToast();
  const { pathname } = useLocation();
  const [busy, setBusy] = useState(false);
  // Signed-out visitors never ask: the endpoint needs a session, and the entry
  // it controls is inside the signed-in half of the bar anyway.
  const { status: ai } = useAIStatus(!!user);
  const aiEnabled = ai?.enabled === true;

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

        <div className="flex items-center gap-2.5 text-xs">
          {user && (
            <>
              <NavItem to="/dashboard" icon="fa-gauge" label={t.nav.overview} active={pathname === "/dashboard"} />
              <NavItem to="/upload" icon="fa-cloud-arrow-up" label={t.common.upload} active={pathname === "/upload"} />
              <NavItem to="/gallery" icon="fa-images" label={t.nav.gallery} active={pathname === "/gallery"} />
              {/* Absent, not disabled, where the deployment configured no AI
                  key: there is nothing behind the link on such a build. */}
              {aiEnabled && (
                <NavItem
                  to="/generate"
                  icon="fa-wand-magic-sparkles"
                  label={t.nav.generate}
                  active={pathname === "/generate"}
                />
              )}
              <NavItem to="/refer" icon="fa-gift" label={t.nav.refer} active={pathname === "/refer"} />
            </>
          )}
          {user?.role === "admin" && (
            <Link to="/admin" className="text-brand-400 hover:underline hidden sm:inline">
              {t.nav.admin}
            </Link>
          )}
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
                    <i className="fa-solid fa-check text-[10px] mr-1" />
                    {t.common.days(user.checkin_streak)}
                  </>
                ) : (
                  <>
                    <i className="fa-solid fa-calendar-check text-[10px] mr-1" />
                    {t.nav.checkin}
                  </>
                )}
              </button>
              <Link
                to="/space"
                title={t.nav.storagePillTitle(formatBytes(user.used_bytes), formatBytes(user.quota_bytes))}
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
                <i className="fa-solid fa-right-from-bracket" />
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
              >
                <i className="fa-solid fa-right-to-bracket text-[10px] mr-1" />
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
      className={`transition ${active ? "text-brand-300" : "text-neutral-400 hover:text-brand-300"}`}
    >
      <i className={`fa-solid ${icon}`} />
      <span className="ml-1.5 hidden md:inline">{label}</span>
    </Link>
  );
}
