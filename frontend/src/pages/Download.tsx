import Footer from "../Footer";
import Icon from "../Icon";
import Nav from "../components/Nav";
import MacDownloadSection, {
  DownloadButtons,
  DownloadMeta,
  GITHUB_RELEASES,
  useMacInfo,
} from "../components/MacDownload";
import Reveal from "../components/Reveal";
import { useLang } from "../LangContext";

export default function Download() {
  const { t } = useLang();
  const m = t.mac;
  const { info, loading } = useMacInfo();

  return (
    <div className="min-h-screen flex flex-col">
      <Nav />
      <main className="flex-1">
        {/* Hero：截图 + 下载 */}
        <section className="max-w-7xl mx-auto px-4 sm:px-6 pt-12 pb-8 sm:pt-16">
          <Reveal>
            <div className="text-center max-w-2xl mx-auto">
              <h1 className="text-2xl sm:text-3xl font-brand text-neutral-100">{m.tagline}</h1>
              <p className="mt-3 text-sm text-neutral-400 leading-relaxed">{m.intro}</p>
              <div className="mt-6 flex justify-center">
                <DownloadButtons info={info} />
              </div>
              <div className="mt-4 flex justify-center">
                <DownloadMeta info={info} />
              </div>
              {info?.minimum_system && (
                <p className="mt-2 text-[11px] text-neutral-600">
                  {m.requires(info.minimum_system)}
                </p>
              )}
              {info?.dmg_url && (
                <p className="mt-3 text-[11px] text-neutral-600 max-w-md mx-auto leading-relaxed">
                  {m.dmgHint}
                </p>
              )}
              {!loading && !info?.version && (
                <p className="mt-3 text-xs text-neutral-500">{m.unavailable}</p>
              )}
            </div>
          </Reveal>

          <Reveal direction="scale" duration={700}>
            <div className="mt-10 rounded-2xl overflow-hidden border border-neutral-800 max-w-5xl mx-auto">
              <img
                src="/img/app-overview.jpg"
                alt={m.sectionTitle}
                width={1600}
                height={1028}
                className="w-full h-auto block"
              />
            </div>
          </Reveal>
        </section>

        {/* 功能要点 */}
        <section className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
          <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-3">
            {m.features.map((f) => (
              <Reveal key={f.title}>
                <div className="h-full rounded-2xl border border-neutral-800 bg-neutral-900/40 p-5">
                  <Icon name={f.icon} className="text-brand-400" />
                  <h3 className="mt-3 text-sm font-medium text-neutral-100">{f.title}</h3>
                  <p className="mt-1.5 text-xs text-neutral-500 leading-relaxed">{f.desc}</p>
                </div>
              </Reveal>
            ))}
          </div>
        </section>

        {/* 更新记录 */}
        <section className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
          <Reveal>
            <div className="mb-5">
              <h2 className="text-lg font-brand text-neutral-100">{m.changelog}</h2>
              <p className="text-xs text-neutral-500 mt-1">{m.changelogSubtitle}</p>
            </div>
          </Reveal>

          {loading && <p className="text-xs text-neutral-600">{m.loading}</p>}

          <div className="space-y-3">
            {(info?.history ?? []).map((r) => (
              <Reveal key={r.version}>
                <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-5">
                  <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                    <span className="text-sm font-medium text-neutral-100 tabular-nums">
                      {r.version}
                    </span>
                    <span className="text-xs text-neutral-500 tabular-nums">{r.date}</span>
                    {r.version === info?.version && (
                      <span className="rounded-md bg-brand-600/20 px-1.5 py-0.5 text-[10px] text-brand-300">
                        {m.version}
                      </span>
                    )}
                    <a
                      href={r.url}
                      target="_blank"
                      rel="noreferrer"
                      className="ml-auto text-[11px] text-neutral-600 hover:text-neutral-400 transition"
                    >
                      {m.fullChangelog}
                      <Icon name="arrow-up-right-from-square" className="ml-1 text-[9px]" />
                    </a>
                  </div>
                  {(r.highlights?.length ?? 0) > 0 && (
                    <ul className="mt-3 space-y-1.5">
                      {(r.highlights ?? []).map((h) => (
                        <li key={h} className="flex gap-2 text-xs text-neutral-400 leading-relaxed">
                          <Icon name="check" className="mt-0.5 shrink-0 text-brand-500 text-[10px]" />
                          <span>{h}</span>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              </Reveal>
            ))}
          </div>

          {(info?.history?.length ?? 0) > 0 && (
            <div className="mt-5 text-center">
              <a
                href={GITHUB_RELEASES}
                target="_blank"
                rel="noreferrer"
                className="text-xs text-neutral-600 hover:text-neutral-400 transition"
              >
                {m.viewOnGithub}
                <Icon name="arrow-up-right-from-square" className="ml-1 text-[9px]" />
              </a>
            </div>
          )}
        </section>
      </main>
      <Footer />
    </div>
  );
}

// 首页那一块从这里复用，避免两处各写一遍。
export { MacDownloadSection };
