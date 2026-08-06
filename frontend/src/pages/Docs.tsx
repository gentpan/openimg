import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import Footer from "../Footer";
import Nav from "../components/Nav";

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
  return (
    <div className="min-h-screen flex flex-col">
      <Nav />
      <main className="mx-auto w-full max-w-4xl flex-1 px-4 sm:px-6 py-8">
        <header className="mb-10">
          <h1 className="text-2xl sm:text-3xl font-brand text-neutral-100">接入 Openimg</h1>
          <p className="mt-2 text-sm text-neutral-500">
            用一枚 API Token，把图床接进 PicGo、Typora、脚本或 CI。
          </p>
        </header>

        <Toc />

        <Section id="token" title="1 · 先拿一枚 Token">
          <P>
            到 <Link to="/settings" className="text-brand-400 hover:underline">设置 → API Token</Link>{" "}
            创建。明文<strong className="text-neutral-300">只显示这一次</strong>，关掉就再也看不到——
            服务端只存哈希，找回不了，丢了只能重建。
          </P>
          <P>
            Token 以 <Code>oimg_</Code> 开头。三种请求头写法都收，服务端按这个顺序取第一个非空的：
          </P>
          <Pre>{`Authorization: Bearer oimg_xxxxxx     # 最常见
Authorization: oimg_xxxxxx            # 裸写，必须以 oimg_ 开头
X-API-Key: oimg_xxxxxx                # 给不会加 Bearer 前缀的客户端`}</Pre>
          <Note>
            第三种是为 PicGo、Typora 这类工具留的——它们能设自定义请求头，但不一定会处理{" "}
            <Code>Bearer</Code> 前缀。不过<strong className="text-neutral-300">浏览器里的脚本别用它</strong>：
            服务端 CORS 放行的请求头只有 Origin、Content-Type、Accept、Authorization，
            <Code>X-API-Key</Code> 会被预检挡掉。
          </Note>
        </Section>

        <Section id="curl" title="2 · 一条命令上传">
          <Pre>{`curl -X POST https://openimg.io/api/upload \\
  -H "Authorization: Bearer oimg_xxxxxx" \\
  -F "file=@photo.jpg"`}</Pre>
          <P>
            两条硬约束：必须是 <Code>multipart/form-data</Code>，字段名必须是{" "}
            <Code>file</Code>。服务端只读这一个字段，质量、格式、宽度这些一律不看——
            转码行为由你账号的设置决定，不由请求决定。
          </P>
          <P>
            成功返回 <Code>201 Created</Code>，结构是这样（只列常用字段）：
          </P>
          <Pre>{`{
  "image": {
    "url":        "https://openimg.io/storage/2026/08/06/aB3dEf7hJ9kL.jpg",
    "short_url":  "https://openimg.io/aB3dE",
    "thumb_url":  "https://openimg.io/storage/2026/08/06/aB3dEf7hJ9kL_w600.webp",
    "markdown":   "![photo.jpg](https://openimg.io/storage/...)",
    "html":       "<img src=\\"https://openimg.io/storage/...\\" alt=\\"photo.jpg\\">",
    "bbcode":     "[img]https://openimg.io/storage/...[/img]",
    "orig_name":  "photo.jpg",
    "width": 3000, "height": 2000,
    "size_orig": 2411520, "size_stored": 1905312,
    "variants":     "webp,w600",
    "variant_urls": { "webp": "...", "w600": "..." }
  },
  "deduplicated": false
}`}</Pre>
          <P>只要直链的话：</P>
          <Pre>{`curl -s -X POST https://openimg.io/api/upload \\
  -H "Authorization: Bearer oimg_xxxxxx" \\
  -F "file=@photo.jpg" | jq -r '.image.url'`}</Pre>
          <Note>
            <strong className="text-neutral-300">图片对象在 <Code>image</Code> 下面</strong>，
            顶层只有 <Code>image</Code> 和 <Code>deduplicated</Code> 两个键。填 JSON 路径的地方
            要写 <Code>image.url</Code>，不是 <Code>url</Code>——这是接入时最常踩的一脚。
          </Note>
          <P>
            <Code>deduplicated: true</Code> 表示服务端已经存过一模一样的字节，走秒传：不重新转码、
            不重新写对象，但<strong className="text-neutral-300">配额照样全额扣</strong>。
          </P>
          <P>
            另外 <Code>variants</Code> 是逗号分隔的<strong className="text-neutral-300">字符串</strong>，
            <Code>variant_urls</Code> 才是对象——两个字段同时存在，容易看混。
          </P>
        </Section>

        <Section id="picgo" title="3 · PicGo">
          <P>
            装插件 <Code>picgo-plugin-web-uploader</Code>，然后逐项填：
          </P>
          <Table
            rows={[
              ["API 地址", "https://openimg.io/api/upload"],
              ["POST 参数名", "file"],
              ["JSON 路径", "image.url"],
              ["自定义请求头", `{"Authorization": "Bearer oimg_xxxxxx"}`],
              ["自定义 Body", "留空"],
            ]}
          />
          <P>
            想让粘贴出来的是短链，把 JSON 路径改成 <Code>image.short_url</Code> 就行。
          </P>
          <Note>
            PicGo 相册里的「删除」只删本地记录，不会真的删服务器上的图——自定义 Web 图床
            没有删除回调。要真删见下面第 6 节。
          </Note>
        </Section>

        <Section id="typora" title="4 · Typora">
          <P>
            <strong className="text-neutral-300">Typora 不能直连。</strong>
            它的上传服务只有 PicGo、iPic、uPic 和「自定义命令」几个选项，没有可以填
            API 地址和请求头的表单。两条路：
          </P>
          <P className="text-neutral-300">走 PicGo（推荐）</P>
          <P>
            按上一节配好 PicGo，然后 Typora → 偏好设置 → 图像 → 上传服务选{" "}
            <Code>PicGo (app)</Code>，指定可执行文件路径，点「验证图片上传选项」。
          </P>
          <P className="text-neutral-300 mt-4">不想装 PicGo：自定义命令</P>
          <P>
            Typora 会把待上传的文件路径作为参数传给命令，并读取输出的最后几行当 URL。
            存成 <Code>~/bin/openimg-upload.sh</Code> 并 <Code>chmod +x</Code>：
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
            然后上传服务选 <Code>Custom Command</Code>，命令填这个脚本的路径。需要本机有{" "}
            <Code>curl</Code> 和 <Code>jq</Code>。
          </P>
        </Section>

        <Section id="errors" title="5 · 出错了怎么办">
          <P>
            错误响应统一是 <Code>{`{"error": "..."}`}</Code>，个别会多带字段。
          </P>
          <ErrorTable />
          <Note>
            <strong className="text-neutral-300">两种 429 要分开处理。</strong>
            只看状态码会误判：带 <Code>retry_after</Code> 的是频率限制（每 IP 每分钟 60 次），
            退避之后可以继续；带 <Code>used</Code> / <Code>limit</Code> 的是当天配额用完了，
            退避没有意义，要等第二天。
          </Note>
        </Section>

        <Section id="limits" title="6 · 限制">
          <P>
            单文件大小、每日次数、允许格式、像素尺寸都按<strong className="text-neutral-300">用户组</strong>
            走，管理员可能调整过。别把数字写死在客户端里，现取：
          </P>
          <Pre>{`curl -s https://openimg.io/api/quota \\
  -H "Authorization: Bearer oimg_xxxxxx" | jq '.tier'`}</Pre>
          <P>
            返回里有 <Code>max_file_size</Code>、<Code>daily_upload_count</Code>、
            <Code>allowed_formats</Code>，以及 <Code>available_bytes</Code> 剩余空间。
          </P>
          <Note>
            大小上限卡的是<strong className="text-neutral-300">整个请求体</strong>，
            不只是文件本身——multipart 的边界和头部也算进去。所以一个字节数刚好等于上限的
            文件仍然会被拒，留 1% 余量比较稳。
          </Note>
        </Section>

        <Section id="more" title="7 · 其余接口">
          <P>这些接口同样用 Token 调：</P>
          <Pre>{`# 列表（分页、搜索、排序）
curl -s "https://openimg.io/api/images?limit=50&offset=0&q=screenshot&sort=newest" \\
  -H "Authorization: Bearer oimg_xxxxxx"

# 单张详情
curl -s https://openimg.io/api/images/<id> \\
  -H "Authorization: Bearer oimg_xxxxxx"

# 删除一张
curl -X DELETE https://openimg.io/api/images/<id> \\
  -H "Authorization: Bearer oimg_xxxxxx"

# 批量删除
curl -X POST https://openimg.io/api/images/bulk-delete \\
  -H "Authorization: Bearer oimg_xxxxxx" \\
  -H "Content-Type: application/json" \\
  -d '{"ids": ["<id1>", "<id2>"]}'

# 每日签到领空间
curl -X POST https://openimg.io/api/checkin \\
  -H "Authorization: Bearer oimg_xxxxxx"`}</Pre>
          <Note>
            改密码、管理 Token、绑定自有存储这些<strong className="text-neutral-300">不能用 Token 调</strong>，
            只能在网页上操作。一枚泄漏的 Token 不该能接管账号——它能传图删图，
            但改不了密码、发不了新 Token、读不到你的 S3 密钥。
          </Note>
        </Section>

        <div className="mt-12 rounded-xl border border-neutral-800 bg-neutral-900/40 p-5 text-sm text-neutral-400">
          文档和代码对不上，或者哪里没写清楚？
          <a
            href="https://github.com/gentpan/openimg/issues"
            target="_blank"
            rel="noopener noreferrer"
            className="ml-1 text-brand-400 hover:underline"
          >
            提个 issue
          </a>
          。
        </div>
      </main>
      <Footer />
    </div>
  );
}

/** Anchors, and a reading order. Long single pages need a way in. */
function Toc() {
  const items = [
    ["token", "先拿一枚 Token"],
    ["curl", "一条命令上传"],
    ["picgo", "PicGo"],
    ["typora", "Typora"],
    ["errors", "出错了怎么办"],
    ["limits", "限制"],
    ["more", "其余接口"],
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
        {copied ? "已复制" : "复制"}
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
  const rows: [string, string, string][] = [
    ["401", "auth: invalid token format", "Token 不以 oimg_ 开头，多半是粘贴时丢了字符"],
    ["401", "auth: invalid token", "查不到这枚 Token —— 已被删除，或者抄错了"],
    ["401", "auth: token expired", "过期了，重建一枚（创建时不填有效期即永不过期）"],
    ["401", "not authenticated", "一个认证头都没带"],
    ["403", "请先验证邮箱后再上传", "带 code: email_unverified，去网页验证邮箱"],
    ["413", "文件超过大小上限 X MB", "压缩后重传"],
    ["400", "缺少上传文件字段 file", "字段名不是 file，或请求不是合法 multipart"],
    ["415", "不支持的图片格式：X", "格式不在白名单。SVG、PDF 明确拒绝"],
    ["415", "当前用户组不允许上传 X 格式", "响应里的 allowed 数组是你能传的格式"],
    ["415", "图片尺寸 AxB 超出上限 CxD", "像素超限。注意是 415 不是 413"],
    ["429", "上传过于频繁，请稍后再试", "带 retry_after 秒数，退避后重试"],
    ["429", "今日上传数量已达上限", "带 used / limit，等第二天"],
    ["507", "insufficient storage quota：…", "空间不足，删图或签到领空间"],
    ["503", "存储不可用：…", "后端存储出问题，稍后重试"],
  ];
  return (
    <div className="overflow-x-auto rounded-xl border border-neutral-800">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-neutral-800 text-left text-xs text-neutral-600">
            <th className="px-4 py-2 font-normal">状态码</th>
            <th className="px-4 py-2 font-normal">error</th>
            <th className="px-4 py-2 font-normal">怎么办</th>
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
