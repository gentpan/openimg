import { useState } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../AuthContext";
import Footer from "../Footer";
import Nav from "../components/Nav";
import Uploader from "../components/Uploader";
import Reveal, { RevealGroup } from "../components/Reveal";
import { useLang } from "../LangContext";
import type { Dict } from "../i18n";

/** Hero feature pills. Six is the ceiling: past that the row wraps to three
 *  lines on a phone and reads as a wall rather than a summary.
 *
 *  Holds dictionary keys rather than text: a module constant is evaluated
 *  once at import, while the language can change at any time after. */
const HIGHLIGHTS: { icon: string; key: keyof Dict["home"]["highlight"] }[] = [
  { icon: "fa-sliders", key: "optimizeOrOriginal" },
  { icon: "fa-file-zipper", key: "webpAvif" },
  { icon: "fa-globe", key: "globalCdn" },
  { icon: "fa-user-shield", key: "stripExif" },
  { icon: "fa-cloud", key: "ownBucket" },
  { icon: "fa-infinity", key: "freeForever" },
];

export default function Home() {
  const { t } = useLang();
  const { user } = useAuth();


  return (
    <div className="min-h-screen flex flex-col bg-neutral-950">
      <Nav />

      <div className="flex-1">
        {/* Hero — the uploader is the hero. Nothing stands between a visitor
            and the thing they came to do. */}
        <section className="max-w-7xl mx-auto px-4 sm:px-6 pt-14 pb-10 sm:pt-20">
          <div className="text-center mb-8">
            <Reveal direction="scale" duration={500}>
              <span className="inline-flex items-center gap-1.5 rounded-full border border-brand-500/30 bg-brand-950/30 px-3 py-1 text-[11px] text-brand-300 mb-5">
                <i className="fa-solid fa-heart text-[9px]" />
                {t.home.hero.badge}
              </span>
            </Reveal>
            <Reveal delay={80}>
              <h1 className="text-3xl sm:text-5xl font-brand text-neutral-100 leading-tight">
                {t.home.hero.titleLead}
                <span className="text-[var(--color-brand-display)]">{t.home.hero.titleAccent}</span>
              </h1>
            </Reveal>
            <Reveal delay={160}>
              <p className="mt-4 text-sm sm:text-base text-neutral-400 max-w-xl mx-auto leading-relaxed">
                {t.home.hero.subtitle}
              </p>
            </Reveal>
            {/* Pills instead of another sentence: these are six independent
                claims, and a prose list makes the reader hold all of them in
                order before any one of them lands. */}
            <Reveal delay={220}>
              <ul className="mt-5 flex flex-wrap items-center justify-center gap-2">
                {HIGHLIGHTS.map((h) => (
                  <li
                    key={h.key}
                    className="inline-flex items-center gap-1.5 rounded-full border border-neutral-800 bg-neutral-900/50 px-3 py-1.5 text-[11px] text-neutral-300"
                  >
                    <i className={`fa-solid ${h.icon} text-[9px] text-brand-400`} />
                    {t.home.highlight[h.key]}
                  </li>
                ))}
              </ul>
            </Reveal>
          </div>

          <Reveal delay={240} duration={700}>
            <Uploader />
          </Reveal>

        </section>

        {/* Features */}
        <section className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
          <Reveal>
            <SectionHead
              title={t.home.features.sectionTitle}
              subtitle={t.home.features.sectionSubtitle}
            />
          </Reveal>
          <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-3">
            <RevealGroup step={70} className="h-full">
              {[
                <Feature
                  key="a"
                  icon="fa-compress"
                  title={t.home.features.optimize.title}
                  desc={t.home.features.optimize.desc}
                />,
                <Feature
                  key="b"
                  icon="fa-bolt"
                  title={t.home.features.cdn.title}
                  desc={t.home.features.cdn.desc}
                />,
                <Feature
                  key="c"
                  icon="fa-shield-halved"
                  title={t.home.features.privacy.title}
                  desc={t.home.features.privacy.desc}
                />,
                <Feature
                  key="d"
                  icon="fa-cloud"
                  title={t.home.features.ownStorage.title}
                  desc={t.home.features.ownStorage.desc}
                />,
                <Feature
                  key="e"
                  icon="fa-calendar-check"
                  title={t.home.features.checkin.title}
                  desc={t.home.features.checkin.desc}
                />,
                <Feature
                  key="f"
                  icon="fa-plug"
                  title={t.home.features.tools.title}
                  desc={t.home.features.tools.desc}
                />,
              ]}
            </RevealGroup>
          </div>
        </section>

        {/* How it works */}
        <section className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
          <Reveal>
            <SectionHead title={t.home.steps.sectionTitle} subtitle={t.home.steps.sectionSubtitle} />
          </Reveal>
          <div className="grid sm:grid-cols-3 gap-3">
            <RevealGroup step={100} className="h-full">
              {[
                <Step
                  key="1"
                  n={1}
                  title={t.home.steps.signUp.title}
                  desc={t.home.steps.signUp.desc}
                />,
                <Step
                  key="2"
                  n={2}
                  title={t.home.steps.drop.title}
                  desc={t.home.steps.drop.desc}
                />,
                <Step
                  key="3"
                  n={3}
                  title={t.home.steps.copy.title}
                  desc={t.home.steps.copy.desc}
                />,
              ]}
            </RevealGroup>
          </div>
        </section>

        {/* Integration */}
        <section className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
          <Reveal>
            <SectionHead
              title={t.home.integration.sectionTitle}
              subtitle={t.home.integration.sectionSubtitle}
            />
          </Reveal>
          <div className="grid lg:grid-cols-2 gap-3">
            <CodeCard
              title={t.home.integration.curlCardTitle}
              copyLabel={t.common.copy}
              copiedLabel={t.common.copied}
              icon="fa-terminal"
              code={`curl -X POST https://openimg.io/api/upload \\
  -H "Authorization: Bearer oimg_xxxxxx" \\
  -F "file=@photo.jpg"`}
            />
            <CodeCard
              title={t.home.integration.picgoCardTitle}
              copyLabel={t.common.copy}
              copiedLabel={t.common.copied}
              icon="fa-plug"
              code={[
                  `${t.home.integration.picgoConfigLabel.apiUrl}   https://openimg.io/api/upload`,
                  `${t.home.integration.picgoConfigLabel.postField}  file`,
                  t.home.integration.picgoConfigLabel.customHeader,
                  `  { "Authorization": "Bearer oimg_xxxxxx" }`,
                  `${t.home.integration.picgoConfigLabel.jsonPath}   image.url`,
                ].join("\n")}
            />
          </div>
        </section>

        {/* CTA */}
        {!user && (
          <section className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
            <Reveal direction="scale" duration={700}>
              <div className="rounded-2xl border border-brand-500/20 bg-brand-950/20 px-6 py-10 text-center">
                <h2 className="text-xl sm:text-2xl font-brand text-neutral-100">
                  {t.home.cta.title}
                </h2>
                <p className="mt-2.5 text-sm text-neutral-400">
                  {t.home.cta.subtitle}
                </p>
                <Link
                  to="/register"
                  className="mt-6 inline-block rounded-xl bg-brand-600 px-6 py-2.5 text-sm font-medium text-brand-ink hover:bg-brand-500 transition"
                >
                  {t.home.cta.button}
                </Link>
              </div>
            </Reveal>
          </section>
        )}
      </div>

      <Footer />
    </div>
  );
}


function SectionHead({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <div className="mb-5">
      <h2 className="text-lg font-brand text-neutral-100">{title}</h2>
      <p className="text-xs text-neutral-500 mt-1">{subtitle}</p>
    </div>
  );
}

function Feature({
  icon,
  title,
  desc,
}: {
  icon: string;
  title: string;
  desc: string;
}) {
  return (
    <div className="h-full rounded-2xl border border-neutral-800 bg-neutral-900/40 p-5 hover:border-neutral-700 transition">
      <div className="w-9 h-9 rounded-lg bg-brand-950/40 flex items-center justify-center mb-3">
        <i className={`fa-solid ${icon} text-brand-400 text-sm`} />
      </div>
      <div className="text-sm font-medium text-neutral-100">{title}</div>
      <p className="mt-1.5 text-xs text-neutral-500 leading-relaxed">{desc}</p>
    </div>
  );
}

function Step({ n, title, desc }: { n: number; title: string; desc: string }) {
  return (
    <div className="h-full rounded-2xl border border-neutral-800 bg-neutral-900/40 p-5">
      <div className="w-7 h-7 rounded-full bg-brand-600 text-brand-ink text-xs font-medium flex items-center justify-center mb-3">
        {n}
      </div>
      <div className="text-sm font-medium text-neutral-100">{title}</div>
      <p className="mt-1.5 text-xs text-neutral-500 leading-relaxed">{desc}</p>
    </div>
  );
}

function CodeCard({
  title,
  icon,
  code,
  copyLabel,
  copiedLabel,
}: {
  title: string;
  icon: string;
  copyLabel: string;
  copiedLabel: string;
  code: string;
}) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 overflow-hidden">
      <div className="flex items-center justify-between px-4 py-2.5 border-b border-neutral-800/60">
        <span className="text-xs text-neutral-300">
          <i className={`fa-solid ${icon} mr-1.5 text-neutral-500`} />
          {title}
        </span>
        <button
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(code);
              setCopied(true);
              setTimeout(() => setCopied(false), 1500);
            } catch {}
          }}
          className="text-[10px] text-neutral-500 hover:text-brand-300 transition"
        >
          <i className={`fa-solid ${copied ? "fa-check" : "fa-copy"} mr-1`} />
          {copied ? copiedLabel : copyLabel}
        </button>
      </div>
      <pre className="px-4 py-3 text-[11px] leading-relaxed text-neutral-400 overflow-x-auto">
        <code>{code}</code>
      </pre>
    </div>
  );
}
