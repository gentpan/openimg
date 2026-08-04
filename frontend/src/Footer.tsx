import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { formatBytes, imageApi } from "./api";
import type { PublicStats } from "./types";

export default function Footer() {
  const [stats, setStats] = useState<PublicStats | null>(null);

  useEffect(() => {
    const load = () => imageApi.publicStats().then(setStats).catch(() => {});
    load();
    const t = setInterval(load, 30_000);
    return () => clearInterval(t);
  }, []);

  return (
    <footer className="mt-16 border-t border-neutral-900 bg-neutral-950/60">
      {stats && (
        <div className="border-b border-neutral-900/70">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 py-4 grid grid-cols-2 sm:grid-cols-4 gap-3">
            <Stat icon="fa-images" label="托管图片" value={stats.total_images.toLocaleString()} />
            <Stat icon="fa-users" label="注册用户" value={stats.total_users.toLocaleString()} />
            <Stat icon="fa-database" label="存储用量" value={formatBytes(stats.stored_bytes, 0)} />
            <Stat icon="fa-arrow-trend-up" label="今日上传" value={stats.images_today.toLocaleString()} />
          </div>
        </div>
      )}

      <div className="max-w-7xl mx-auto px-4 sm:px-6 py-5 grid grid-cols-2 sm:grid-cols-3 items-center text-xs text-neutral-500">
        <div className="text-left">
          © {new Date().getFullYear()}{" "}
          {/* Router link rather than an absolute URL: on openimg.io this *is*
              openimg.io, and it avoids a full page reload. */}
          <Link to="/" className="font-brand hover:opacity-80 transition-opacity">
            <span className="text-neutral-100">Open</span>
            <span className="text-violet-400">img</span>
          </Link>
          <span className="ml-1.5 text-neutral-600">All Rights Reserved.</span>
        </div>
        <div className="hidden sm:block text-center text-[11px] text-faint">
          永久免费 · 无广告 · 不出售数据
        </div>
        <div className="flex items-center justify-end gap-4">
          {SOCIALS.map((s) => (
            <a
              key={s.label}
              href={s.href}
              target="_blank"
              rel="noopener noreferrer"
              aria-label={s.label}
              title={s.label}
              // The box stays 5×5 and only the glyph scales, so growing one
              // icon never nudges its neighbours along the row.
              className={`flex h-5 w-5 items-center justify-center text-neutral-500 transition-[color,transform] duration-200 hover:scale-150 ${
                s.hover ?? "hover:text-violet-300"
              }`}
            >
              {s.icon}
            </a>
          ))}
        </div>
      </div>
    </footer>
  );
}

const SOCIALS: { label: string; href: string; icon: React.ReactNode; hover?: string }[] = [
  { label: "X", href: "https://x.com/giantaccel", icon: <XIcon /> },
  { label: "GitHub", href: "https://github.com/gentpan/openimg", icon: <GithubIcon /> },
  { label: "Blog", href: "https://xifeng.net", icon: <BlogIcon /> },
  {
    label: "GiantAccel",
    href: "https://www.giantaccel.com",
    icon: <GiantAccelIcon />,
    // Brand blue, written as a literal so Tailwind can extract it at build time.
    hover: "hover:text-[#0052D9]",
  },
];

function Stat({ icon, label, value }: { icon: string; label: string; value: string }) {
  return (
    <div className="flex items-center gap-2.5">
      <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-neutral-900 text-violet-400">
        <i className={`fa-solid ${icon} text-[11px]`} />
      </span>
      <span className="min-w-0">
        <span className="block text-[10px] text-neutral-600">{label}</span>
        <span className="block text-sm font-brand text-neutral-100 tabular-nums leading-tight">{value}</span>
      </span>
    </div>
  );
}

function XIcon() {
  return (
    <svg viewBox="1.25 1.25 21.5 21.5" className="h-4 w-4 fill-current" aria-hidden="true">
      <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
    </svg>
  );
}

function GithubIcon() {
  return (
    <svg viewBox="0 0 16 16" className="h-4 w-4 fill-current" aria-hidden="true">
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0 0 16 8c0-4.42-3.58-8-8-8z" />
    </svg>
  );
}

/**
 * GiantAccel mark. The source artwork sits in the middle two-thirds of its
 * 1024 canvas, so at a shared h-4 it rendered ~11px against the others' 16px.
 * The viewBox is tightened to the actual path bounds (x 168.96–855.04,
 * y 216.06–851.46, squared and centred) and then padded back out ~10%.
 *
 * Two separate levers matter here, and only fixing one leaves it looking
 * wrong: the box size, and how much of that box the ink fills. The other three
 * marks carry their own margin inside their artwork, so a tightened viewBox at
 * the same box size renders visibly larger ink even though the <svg> elements
 * measure identically.
 */
function GiantAccelIcon() {
  return (
    <svg
      viewBox="134.66 156.42 754.69 754.69"
      className="h-4 w-4 fill-current"
      aria-hidden="true"
    >
      <path d="M512 851.456c187.904 0 340.992-152.064 343.04-339.456h-145.408c-2.048 107.52-89.6 194.048-197.632 194.048S316.416 619.52 314.368 512H168.96c2.048 187.392 155.136 339.456 343.04 339.456zM550.912 216.064H855.04v145.408h-304.128z" />
    </svg>
  );
}

/**
 * Blog mark. The source artwork hard-codes #272636; swapped to fill-current so
 * it inherits the link colour and picks up the hover state like its siblings.
 */
function BlogIcon() {
  return (
    <svg viewBox="0 0 1024 1024" className="h-4 w-4 fill-current" aria-hidden="true">
      <path d="M630.816594 591.217203 393.202848 591.217203c-21.781072 0-39.603996 17.819854-39.603996 39.600927s17.822924 39.600927 39.603996 39.600927l237.613746 0c21.781072 0 39.60195-17.819854 39.60195-39.600927S652.597667 591.217203 630.816594 591.217203z" />
      <path d="M393.202848 432.80531l118.80585 0c21.781072 0 39.60195-17.820877 39.60195-39.60195 0-21.782096-17.820877-39.602973-39.60195-39.602973l-118.80585 0c-21.781072 0-39.603996 17.820877-39.603996 39.602973C353.598852 414.984433 371.421776 432.80531 393.202848 432.80531z" />
      <path d="M511.999488 0.606821c-282.434557 0-511.393179 228.958622-511.393179 511.393179 0 282.433534 228.958622 511.395226 511.393179 511.395226s511.393179-228.960669 511.393179-511.395226C1023.392668 229.565443 794.434046 0.606821 511.999488 0.606821zM828.82839 624.287389c0 113.0211-91.996251 204.523093-205.813482 204.523093L401.222499 828.810482c-113.737414 0-206.051913-91.501994-206.051913-204.523093L195.170586 399.795499c0.078795-113.025193 92.314499-204.603935 206.051913-204.603935L505.076822 195.191564c113.817232 0 204.942649 84.986603 204.942649 198.011796 1.486864 21.208021 20.555152 39.60195 42.315758 39.60195l35.523051 0c22.750143 0 40.97011 23.860431 40.97011 46.454008L828.82839 624.287389z" />
    </svg>
  );
}
