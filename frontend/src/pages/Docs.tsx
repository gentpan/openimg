import { Fragment, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import Footer from "../Footer";
import Nav from "../components/Nav";
import { useLang } from "../LangContext";

/**
 * How to put Openimg into a workflow.
 *
 * Public, and deliberately not behind the login wall: someone deciding whether
 * to use the service needs to see what integrating costs before they sign up.
 *
 * Every value on this page is copied from the handlers rather than remembered —
 * the multipart field name, the 201, the shape of the response, the wording of
 * each error. Documentation that disagrees with the API is worse than none,
 * because it costs the reader an hour before they stop believing it.
 */
export default function DocsPage() {
  const { t } = useLang();

  return (
    <div className="min-h-screen flex flex-col">
      <Nav />
      <main className="mx-auto w-full max-w-4xl flex-1 px-4 sm:px-6 py-8">
        <header className="mb-10">
          <h1 className="text-2xl sm:text-3xl font-brand text-neutral-100">{t.docs.title}</h1>
          <p className="mt-2 text-sm text-neutral-500">
            {t.docs.subtitle}
          </p>
        </header>

        <Toc />

        <Section id="token" title={`1 · ${t.docs.token.heading}`}>
          <P>
            <Fill text={t.docs.token.createIn}>
              {[
                <Link to="/settings" className="text-brand-400 hover:underline">
                  {t.docs.token.settingsLink}
                </Link>,
                <strong className="text-neutral-300">{t.docs.token.onceEmphasis}</strong>,
              ]}
            </Fill>
          </P>
          <P>
            <Fill text={t.docs.token.headerForms}>{[<Code>oimg_</Code>]}</Fill>
          </P>
          <Pre>{`Authorization: Bearer oimg_xxxxxx     # ${t.docs.token.commentCommon}
Authorization: oimg_xxxxxx            # ${t.docs.token.commentBare}
X-API-Key: oimg_xxxxxx                # ${t.docs.token.commentNoBearer}`}</Pre>
          <Note>
            <Fill text={t.docs.token.note}>
              {[
                <Code>Bearer</Code>,
                <strong className="text-neutral-300">{t.docs.token.noteWarn}</strong>,
                <Code>X-API-Key</Code>,
              ]}
            </Fill>
          </Note>
        </Section>

        <Section id="curl" title={`2 · ${t.docs.curl.heading}`}>
          <Pre>{`curl -X POST https://openimg.io/api/upload \\
  -H "Authorization: Bearer oimg_xxxxxx" \\
  -F "file=@photo.jpg"`}</Pre>
          <P>
            <Fill text={t.docs.curl.constraints}>
              {[<Code>multipart/form-data</Code>, <Code>file</Code>]}
            </Fill>
          </P>
          <P>
            <Fill text={t.docs.curl.success}>{[<Code>201 Created</Code>]}</Fill>
          </P>
          <Pre>{`{
  "image": {
    "url":        "https://openimg.io/storage/2026/08/06/aB3dEf7hJ9kL.webp",
    "short_url":  "https://openimg.io/aB3dE",
    "thumb_url":  "https://openimg.io/storage/2026/08/06/aB3dEf7hJ9kL_w600.webp",
    "markdown":   "![photo.jpg](https://openimg.io/storage/...)",
    "html":       "<img src=\\"https://openimg.io/storage/...\\" alt=\\"photo.jpg\\">",
    "bbcode":     "[img]https://openimg.io/storage/...[/img]",
    "orig_name":  "photo.jpg",
    "width": 3000, "height": 2000,
    "size_orig": 2411520, "size_stored": 812544,
    "variants":     "w600",
    "variant_urls": { "w600": "..." }
  },
  "deduplicated": false
}`}</Pre>
          <P>{t.docs.curl.directLinkOnly}</P>
          <Pre>{`curl -s -X POST https://openimg.io/api/upload \\
  -H "Authorization: Bearer oimg_xxxxxx" \\
  -F "file=@photo.jpg" | jq -r '.image.url'`}</Pre>
          <Note>
            <strong className="text-neutral-300">
              <Fill text={t.docs.curl.noteEmphasis}>{[<Code>image</Code>]}</Fill>
            </strong>
            {". "}
            <Fill text={t.docs.curl.note}>
              {[<Code>image</Code>, <Code>deduplicated</Code>, <Code>image.url</Code>, <Code>url</Code>]}
            </Fill>
          </Note>
          <P>
            <Fill text={t.docs.curl.dedup}>
              {[
                <Code>deduplicated: true</Code>,
                <strong className="text-neutral-300">{t.docs.curl.dedupEmphasis}</strong>,
              ]}
            </Fill>
          </P>
          <P>
            <Fill text={t.docs.curl.variants}>
              {[
                <Code>variants</Code>,
                <strong className="text-neutral-300">{t.docs.curl.variantsEmphasis}</strong>,
                <Code>variant_urls</Code>,
              ]}
            </Fill>
          </P>
        </Section>

        <Section id="picgo" title={`3 · ${t.docs.picgo.heading}`}>
          <P>
            <Fill text={t.docs.picgo.install}>{[<Code>picgo-plugin-web-uploader</Code>]}</Fill>
          </P>
          <Table
            rows={[
              [t.docs.picgo.fieldApiUrl, "https://openimg.io/api/upload"],
              [t.docs.picgo.fieldPostName, "file"],
              [t.docs.picgo.fieldJsonPath, "image.url"],
              [t.docs.picgo.fieldHeaders, `{"Authorization": "Bearer oimg_xxxxxx"}`],
              [t.docs.picgo.fieldBody, t.docs.picgo.valueEmpty],
            ]}
          />
          <P>
            <Fill text={t.docs.picgo.shortLink}>{[<Code>image.short_url</Code>]}</Fill>
          </P>
          <Note>{t.docs.picgo.note}</Note>
        </Section>

        <Section id="typora" title={`4 · ${t.docs.typora.heading}`}>
          <P>
            <Fill text={t.docs.typora.intro}>
              {[<strong className="text-neutral-300">{t.docs.typora.introEmphasis}</strong>]}
            </Fill>
          </P>
          <P className="text-neutral-300">{t.docs.typora.viaPicgo}</P>
          <P>
            <Fill text={t.docs.typora.viaPicgoSteps}>{[<Code>PicGo (app)</Code>]}</Fill>
          </P>
          <P className="text-neutral-300 mt-4">{t.docs.typora.viaScript}</P>
          <P>
            <Fill text={t.docs.typora.viaScriptIntro}>
              {[<Code>~/bin/openimg-upload.sh</Code>, <Code>chmod +x</Code>]}
            </Fill>
          </P>
          <Pre>{`#!/usr/bin/env bash
set -euo pipefail
TOKEN="oimg_xxxxxx"
echo "Upload Success:"
for f in "$@"; do
  curl -s -X POST https://openimg.io/api/upload \\
    -H "Authorization: Bearer $TOKEN" \\
    -F "file=@$f" | jq -r '.image.url'
done`}</Pre>
          <P>
            <Fill text={t.docs.typora.viaScriptTail}>
              {[<Code>Custom Command</Code>, <Code>curl</Code>, <Code>jq</Code>]}
            </Fill>
          </P>
        </Section>

        <Section id="errors" title={`5 · ${t.docs.errors.heading}`}>
          <P>
            <Fill text={t.docs.errors.shape}>{[<Code>{`{"error": "..."}`}</Code>]}</Fill>
          </P>
          <ErrorTable />
          <Note>
            <Fill text={t.docs.errors.note}>
              {[
                <strong className="text-neutral-300">{t.docs.errors.noteEmphasis}</strong>,
                <Code>retry_after</Code>,
                <Code>used</Code>,
                <Code>limit</Code>,
              ]}
            </Fill>
          </Note>
        </Section>

        <Section id="limits" title={`6 · ${t.docs.limits.heading}`}>
          <P>
            <Fill text={t.docs.limits.intro}>
              {[<strong className="text-neutral-300">{t.docs.limits.introEmphasis}</strong>]}
            </Fill>
          </P>
          <Pre>{`curl -s https://openimg.io/api/quota \\
  -H "Authorization: Bearer oimg_xxxxxx" | jq '.tier'`}</Pre>
          <P>
            <Fill text={t.docs.limits.fields}>
              {[
                <Code>max_file_size</Code>,
                <Code>daily_upload_count</Code>,
                <Code>allowed_formats</Code>,
                <Code>available_bytes</Code>,
              ]}
            </Fill>
          </P>
          <Note>
            <Fill text={t.docs.limits.note}>
              {[<strong className="text-neutral-300">{t.docs.limits.noteEmphasis}</strong>]}
            </Fill>
          </Note>
        </Section>

        <Section id="more" title={`7 · ${t.docs.more.heading}`}>
          <P>{t.docs.more.intro}</P>
          <Pre>{`# ${t.docs.more.commentList}
curl -s "https://openimg.io/api/images?limit=50&offset=0&q=screenshot&sort=newest" \\
  -H "Authorization: Bearer oimg_xxxxxx"

# ${t.docs.more.commentDetail}
curl -s https://openimg.io/api/images/<id> \\
  -H "Authorization: Bearer oimg_xxxxxx"

# ${t.docs.more.commentDelete}
curl -X DELETE https://openimg.io/api/images/<id> \\
  -H "Authorization: Bearer oimg_xxxxxx"

# ${t.docs.more.commentBulk}
curl -X POST https://openimg.io/api/images/bulk-delete \\
  -H "Authorization: Bearer oimg_xxxxxx" \\
  -H "Content-Type: application/json" \\
  -d '{"ids": ["<id1>", "<id2>"]}'

# ${t.docs.more.commentCheckin}
curl -X POST https://openimg.io/api/checkin \\
  -H "Authorization: Bearer oimg_xxxxxx"`}</Pre>
          <Note>
            <Fill text={t.docs.more.note}>
              {[<strong className="text-neutral-300">{t.docs.more.noteEmphasis}</strong>]}
            </Fill>
          </Note>
        </Section>

        <div className="mt-12 rounded-xl border border-neutral-800 bg-neutral-900/40 p-5 text-sm text-neutral-400">
          <Fill text={t.docs.feedback}>
            {[
              <a
                href="https://github.com/gentpan/openimg/issues"
                target="_blank"
                rel="noopener noreferrer"
                className="text-brand-400 hover:underline"
              >
                {t.docs.feedbackLink}
              </a>,
            ]}
          </Fill>
        </div>
      </main>
      <Footer />
    </div>
  );
}

/** Anchors, and a reading order. Long single pages need a way in. */
function Toc() {
  const { t } = useLang();
  const items: [string, string][] = [
    ["token", t.docs.toc.token],
    ["curl", t.docs.toc.curl],
    ["picgo", t.docs.toc.picgo],
    ["typora", t.docs.toc.typora],
    ["errors", t.docs.toc.errors],
    ["limits", t.docs.toc.limits],
    ["more", t.docs.toc.more],
  ];
  return (
    <nav className="mb-10 flex flex-wrap gap-x-4 gap-y-2 rounded-xl border border-neutral-800 bg-neutral-900/40 px-4 py-3 text-xs">
      {items.map(([id, label], i) => (
        <a key={id} href={`#${id}`} className="text-neutral-400 hover:text-brand-300 transition">
          <span className="mr-1.5 text-neutral-600 tabular-nums">{i + 1}</span>
          {label}
        </a>
      ))}
    </nav>
  );
}

function Section({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    // scroll-mt so an anchor jump doesn't tuck the heading under the sticky nav.
    <section id={id} className="mb-12 scroll-mt-20">
      <h2 className="mb-4 text-lg font-brand text-neutral-100">{title}</h2>
      <div className="space-y-3">{children}</div>
    </section>
  );
}

function P({ children, className = "" }: { children: React.ReactNode; className?: string }) {
  return <p className={`text-sm leading-relaxed text-neutral-400 ${className}`}>{children}</p>;
}

function Code({ children }: { children: React.ReactNode }) {
  return (
    <code className="rounded bg-neutral-900 px-1.5 py-0.5 text-[0.85em] text-brand-300">
      {children}
    </code>
  );
}

function Pre({ children }: { children: string }) {
  const { t } = useLang();
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const t = setTimeout(() => setCopied(false), 1600);
    return () => clearTimeout(t);
  }, [copied]);
  return (
    <div className="group relative">
      <pre className="overflow-x-auto rounded-xl border border-neutral-800 bg-neutral-950 p-4 text-xs leading-relaxed text-neutral-300">
        <code>{children}</code>
      </pre>
      <button
        onClick={() => {
          navigator.clipboard?.writeText(children).then(() => setCopied(true));
        }}
        className="absolute right-2 top-2 rounded-lg bg-neutral-900 px-2 py-1 text-[10px] text-neutral-500 opacity-0 transition group-hover:opacity-100 hover:text-brand-300"
      >
        {copied ? t.common.copied : t.common.copy}
      </button>
    </div>
  );
}

function Note({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-lg border-l-2 border-brand-600 bg-neutral-900/50 px-4 py-3 text-sm leading-relaxed text-neutral-400">
      {children}
    </div>
  );
}

/**
 * A sentence with elements embedded in it.
 *
 * The dictionary entry is a function that interpolates sentinels; this splits on
 * them and drops the elements in. That is what lets the two languages put an
 * element in different places — Chinese says 「到 X 创建」 where English says
 * "Create one in X" — which a fixed sequence of fragments cannot express.
 */
const SLOTS = ["\u0000", "\u0001", "\u0002", "\u0003"];

function Fill({
  text,
  children,
}: {
  text: (...slots: string[]) => string;
  children: React.ReactNode[];
}) {
  const parts = text(...SLOTS.slice(0, children.length)).split(/([\u0000-\u0003])/);
  return (
    <>
      {parts.map((part, i) => {
        const slot = SLOTS.indexOf(part);
        return slot === -1 ? part : <Fragment key={i}>{children[slot]}</Fragment>;
      })}
    </>
  );
}

function Table({ rows }: { rows: [string, string][] }) {
  return (
    <div className="overflow-x-auto rounded-xl border border-neutral-800">
      <table className="w-full text-sm">
        <tbody>
          {rows.map(([k, v], i) => (
            <tr key={k} className={i > 0 ? "border-t border-neutral-800" : ""}>
              <td className="w-40 px-4 py-2.5 align-top text-neutral-500">{k}</td>
              <td className="px-4 py-2.5 font-mono text-xs text-neutral-300">{v}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** Copied from the handlers, wording included. */
function ErrorTable() {
  const { t } = useLang();
  const rows = t.docs.errorRows;

  return (
    <div className="overflow-x-auto rounded-xl border border-neutral-800">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-neutral-800 text-left text-xs text-neutral-600">
            <th className="px-4 py-2 font-normal">{t.docs.errors.colStatus}</th>
            <th className="px-4 py-2 font-normal">error</th>
            <th className="px-4 py-2 font-normal">{t.docs.errors.colFix}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map(([code, msg, fix], i) => (
            <tr key={i} className={i > 0 ? "border-t border-neutral-800/60" : ""}>
              <td className="px-4 py-2.5 align-top font-mono text-xs text-neutral-500 tabular-nums">
                {code}
              </td>
              <td className="px-4 py-2.5 align-top font-mono text-xs text-neutral-300">{msg}</td>
              <td className="px-4 py-2.5 align-top text-xs text-neutral-500">{fix}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
