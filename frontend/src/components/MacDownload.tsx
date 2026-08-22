import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import Icon from "../Icon";
import { appApi, formatBytes } from "../api";
import { useLang } from "../LangContext";
import type { MacInfo } from "../types";

/** 取一次下载页数据。
 *
 * 拿不到不当错误处理:版本信息里有一半来自 GitHub,而首页不该因为 api.github.com
 * 抽风就少一块。取不到时组件退回一个"直接去 GitHub 下载"的链接。
 */
export function useMacInfo() {
  const [info, setInfo] = useState<MacInfo | null>(null);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    let alive = true;
    appApi
      .macInfo()
      .then((d) => alive && setInfo(d))
      .catch(() => {})
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, []);
  return { info, loading };
}

export const GITHUB_RELEASES = "https://github.com/gentpan/openimg-app/releases";

/** 主下载按钮。优先给 dmg —— zip 解压后就地双击会触发 App Translocation,
 *  那种状态下应用内自我更新会永久失效。 */
export function DownloadButtons({ info, compact }: { info: MacInfo | null; compact?: boolean }) {
  const { t } = useLang();
  const m = t.mac;
  const primary = info?.dmg_url || info?.zip_url;

  if (!primary) {
    return (
      <a
        href={GITHUB_RELEASES}
        target="_blank"
        rel="noreferrer"
        className="inline-flex items-center gap-2 rounded-xl bg-brand-600 px-5 py-2.5 text-sm font-medium text-brand-ink hover:bg-brand-500 transition"
      >
        <Icon name="arrow-up-right-from-square" />
        {m.viewOnGithub}
      </a>
    );
  }

  return (
    <div className="flex flex-wrap items-center gap-2">
      <a
        href={primary}
        className="inline-flex items-center gap-2 rounded-xl bg-brand-600 px-5 py-2.5 text-sm font-medium text-brand-ink hover:bg-brand-500 transition"
      >
        <Icon name="download" />
        {info.dmg_url ? m.downloadDmg : m.download}
      </a>
      {info.dmg_url && info.zip_url && !compact && (
        <a
          href={info.zip_url}
          className="inline-flex items-center gap-2 rounded-xl border border-neutral-700 px-4 py-2.5 text-sm text-neutral-300 hover:border-neutral-600 hover:text-neutral-100 transition"
        >
          <Icon name="file-zipper" />
          {m.downloadZip}
        </a>
      )}
    </div>
  );
}

/** 版本、体积、下载次数、更新日期。四个数字排一行。 */
export function DownloadMeta({ info }: { info: MacInfo | null }) {
  const { t } = useLang();
  const m = t.mac;
  if (!info?.version) return null;
  const date = info.published_at ? info.published_at.slice(0, 10) : "";
  return (
    <div className="flex flex-wrap items-center gap-x-5 gap-y-1.5 text-xs text-neutral-500 tabular-nums">
      <span>
        {m.version} <span className="text-neutral-300">{info.version}</span>
      </span>
      {date && (
        <span>
          {m.updated} <span className="text-neutral-300">{date}</span>
        </span>
      )}
      {info.size > 0 && (
        <span>
          {m.size} <span className="text-neutral-300">{formatBytes(info.size, 1)}</span>
        </span>
      )}
      {info.downloads > 0 && (
        <span>
          {m.downloads} <span className="text-neutral-300">{m.times(info.downloads)}</span>
        </span>
      )}
    </div>
  );
}

/** 首页那一块。截图占右半边,信息和按钮在左。 */
export default function MacDownloadSection() {
  const { t } = useLang();
  const m = t.mac;
  const { info } = useMacInfo();

  return (
    <div className="grid lg:grid-cols-2 gap-6 items-center rounded-2xl border border-neutral-800 bg-neutral-900/40 p-6 sm:p-8">
      <div>
        <h3 className="text-xl font-brand text-neutral-100">{m.tagline}</h3>
        <p className="mt-2 text-sm text-neutral-400 leading-relaxed">{m.intro}</p>

        <div className="mt-5">
          <DownloadButtons info={info} compact />
        </div>
        <div className="mt-3">
          <DownloadMeta info={info} />
        </div>
        {info?.minimum_system && (
          <p className="mt-2 text-[11px] text-neutral-600">{m.requires(info.minimum_system)}</p>
        )}

        <Link
          to="/download"
          className="mt-4 inline-flex items-center gap-1.5 text-xs text-brand-400 hover:underline"
        >
          {m.changelog}
          <Icon name="arrow-right" className="text-[10px]" />
        </Link>
      </div>

      <div className="rounded-xl overflow-hidden border border-neutral-800">
        <img
          src="/img/app-overview.jpg"
          alt={m.sectionTitle}
          loading="lazy"
          width={1600}
          height={1028}
          className="w-full h-auto block"
        />
      </div>
    </div>
  );
}
