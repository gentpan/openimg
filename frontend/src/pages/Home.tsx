import { useState } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../AuthContext";
import Footer from "../Footer";
import Nav from "../components/Nav";
import Uploader from "../components/Uploader";
import Reveal, { RevealGroup } from "../components/Reveal";

/** Hero feature pills. Six is the ceiling: past that the row wraps to three
 *  lines on a phone and reads as a wall rather than a summary. */
const HIGHLIGHTS: { icon: string; text: string }[] = [
  { icon: "fa-sliders", text: "压缩或留原图" },
  { icon: "fa-file-zipper", text: "WebP / AVIF" },
  { icon: "fa-globe", text: "全球 CDN" },
  { icon: "fa-user-shield", text: "自动抹除 EXIF" },
  { icon: "fa-cloud", text: "可绑自有 R2 / S3" },
  { icon: "fa-infinity", text: "永久免费" },
];

export default function Home() {
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
              <span className="inline-flex items-center gap-1.5 rounded-full border border-violet-500/30 bg-violet-950/30 px-3 py-1 text-[11px] text-violet-300 mb-5">
                <i className="fa-solid fa-heart text-[9px]" />
                永久免费的公益图床
              </span>
            </Reveal>
            <Reveal delay={80}>
              <h1 className="text-3xl sm:text-5xl font-brand text-neutral-100 leading-tight">
                图片托管，
                <span className="text-violet-400">从此不用操心</span>
              </h1>
            </Reveal>
            <Reveal delay={160}>
              <p className="mt-4 text-sm sm:text-base text-neutral-400 max-w-xl mx-auto leading-relaxed">
                拖进来就好。剩下的压缩、转码、分发，我们全包了。
              </p>
            </Reveal>
            {/* Pills instead of another sentence: these are six independent
                claims, and a prose list makes the reader hold all of them in
                order before any one of them lands. */}
            <Reveal delay={220}>
              <ul className="mt-5 flex flex-wrap items-center justify-center gap-2">
                {HIGHLIGHTS.map((h) => (
                  <li
                    key={h.text}
                    className="inline-flex items-center gap-1.5 rounded-full border border-neutral-800 bg-neutral-900/50 px-3 py-1.5 text-[11px] text-neutral-300"
                  >
                    <i className={`fa-solid ${h.icon} text-[9px] text-violet-400`} />
                    {h.text}
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
              title="为什么用 Openimg"
              subtitle="做好一件事：让图片又小又快"
            />
          </Reveal>
          <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-3">
            <RevealGroup step={70} className="h-full">
              {[
                <Feature
                  key="a"
                  icon="fa-compress"
                  title="自动压缩转换"
                  desc="上传即生成 WebP / AVIF 与多档缩略图，体积通常只剩三分之一，画质肉眼无差。"
                />,
                <Feature
                  key="b"
                  icon="fa-bolt"
                  title="Cloudflare CDN"
                  desc="内容寻址的不可变地址，边缘缓存一年。国内外访问都走最近的节点。"
                />,
                <Feature
                  key="c"
                  icon="fa-shield-halved"
                  title="隐私优先"
                  desc="自动剥离 EXIF 与 GPS 定位信息，重新编码去掉一切夹带内容，不会泄露你的拍摄地点。"
                />,
                <Feature
                  key="d"
                  icon="fa-cloud"
                  title="绑定自有存储"
                  desc="可以把图片直接存进你自己的 Cloudflare R2 或 S3，容量不受平台限制，随时可迁走。"
                />,
                <Feature
                  key="e"
                  icon="fa-calendar-check"
                  title="签到得空间"
                  desc="注册即送 1 GB，每天签到随机领 1–20 MB，连续 7 天额外奖励，邀请好友双方各得 200 MB。容量永久累加。"
                />,
                <Feature
                  key="f"
                  icon="fa-plug"
                  title="工具直连"
                  desc="生成 API Token 后可直接在 PicGo、Typora 或 curl 里使用，写文章时无需切换页面。"
                />,
              ]}
            </RevealGroup>
          </div>
        </section>

        {/* How it works */}
        <section className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
          <Reveal>
            <SectionHead title="三步开始" subtitle="不到一分钟" />
          </Reveal>
          <div className="grid sm:grid-cols-3 gap-3">
            <RevealGroup step={100} className="h-full">
              {[
                <Step
                  key="1"
                  n={1}
                  title="注册账号"
                  desc="邮箱、Google、GitHub 或 Passkey，验证邮箱后即可上传。"
                />,
                <Step
                  key="2"
                  n={2}
                  title="拖进图片"
                  desc="拖拽、点击选择，或直接 Ctrl+V 粘贴截图，支持批量。"
                />,
                <Step
                  key="3"
                  n={3}
                  title="复制链接"
                  desc="一键复制 URL / Markdown / HTML / BBCode，粘到任何地方。"
                />,
              ]}
            </RevealGroup>
          </div>
        </section>

        {/* Integration */}
        <section className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
          <Reveal>
            <SectionHead
              title="接入你的工作流"
              subtitle="在设置页生成 API Token 后即可使用"
            />
          </Reveal>
          <div className="grid lg:grid-cols-2 gap-3">
            <CodeCard
              title="curl"
              icon="fa-terminal"
              code={`curl -X POST https://openimg.io/api/upload \\
  -H "Authorization: Bearer oimg_xxxxxx" \\
  -F "file=@photo.jpg"`}
            />
            <CodeCard
              title="PicGo 自定义图床"
              icon="fa-plug"
              code={`API 地址   https://openimg.io/api/upload
POST 参数名  file
自定义 Header
  { "Authorization": "Bearer oimg_xxxxxx" }
JSON 路径   image.url`}
            />
          </div>
        </section>

        {/* CTA */}
        {!user && (
          <section className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
            <Reveal direction="scale" duration={700}>
              <div className="rounded-2xl border border-violet-500/20 bg-violet-950/20 px-6 py-10 text-center">
                <h2 className="text-xl sm:text-2xl font-brand text-neutral-100">
                  现在就开始，一分钱都不用花
                </h2>
                <p className="mt-2.5 text-sm text-neutral-400">
                  这是一个公益项目，没有会员，没有广告，也不会卖你的数据。
                </p>
                <Link
                  to="/register"
                  className="mt-6 inline-block rounded-xl bg-violet-600 px-6 py-2.5 text-sm font-medium text-white hover:bg-violet-500 transition"
                >
                  免费注册，领 1 GB
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
      <div className="w-9 h-9 rounded-lg bg-violet-950/40 flex items-center justify-center mb-3">
        <i className={`fa-solid ${icon} text-violet-400 text-sm`} />
      </div>
      <div className="text-sm font-medium text-neutral-100">{title}</div>
      <p className="mt-1.5 text-xs text-neutral-500 leading-relaxed">{desc}</p>
    </div>
  );
}

function Step({ n, title, desc }: { n: number; title: string; desc: string }) {
  return (
    <div className="h-full rounded-2xl border border-neutral-800 bg-neutral-900/40 p-5">
      <div className="w-7 h-7 rounded-full bg-violet-600 text-white text-xs font-medium flex items-center justify-center mb-3">
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
}: {
  title: string;
  icon: string;
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
          className="text-[10px] text-neutral-500 hover:text-violet-300 transition"
        >
          <i className={`fa-solid ${copied ? "fa-check" : "fa-copy"} mr-1`} />
          {copied ? "已复制" : "复制"}
        </button>
      </div>
      <pre className="px-4 py-3 text-[11px] leading-relaxed text-neutral-400 overflow-x-auto">
        <code>{code}</code>
      </pre>
    </div>
  );
}
