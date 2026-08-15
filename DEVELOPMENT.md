# 开发文档

永久免费的公益图床 · [openimg.io](https://openimg.io)。本文档覆盖技术栈、开发习惯、部署与进度；
深度架构设计（上传流水线、存储路由、账本、短链）见 [ARCHITECTURE.md](ARCHITECTURE.md)，
macOS/iOS 客户端在独立仓库 [gentpan/openimg-app](https://github.com/gentpan/openimg-app)。

三个交付物在两个仓库里：

| 交付物 | 位置 | 说明 |
| --- | --- | --- |
| 网站（前端 + 后端） | 本仓库 `frontend/` `backend/` | openimg.io 主站 |
| macOS 客户端 | 独立仓库 [gentpan/openimg-app](https://github.com/gentpan/openimg-app) | 从本仓库 `apple/` 拆出（保留历史），发布走其 Releases |
| LiteZoom 灯箱库 | 独立仓库 [gentpan/litezoom](https://github.com/gentpan/litezoom) | 官网 litezoom.dev，本仓库持有同步副本 |

## 技术栈

### 后端（`backend/`，Go 1.26）

| 层面 | 选型 | 备注 |
| --- | --- | --- |
| Web 框架 | Gin v1.10 + gin-contrib/cors | |
| ORM | GORM v1.31 + PostgreSQL（pgx v5） | 启动时 AutoMigrate 全部 13 个模型 |
| 图像处理 | govips v2.18（libvips 绑定，**cgo**） | 不能 `CGO_ENABLED=0` 构建 |
| 对象存储 | aws-sdk-go-v2 S3 | 兼容 MinIO / R2 / B2；本地磁盘开发回退 |
| 鉴权 | golang-jwt v5、go-webauthn（Passkey）、x/oauth2（Google/GitHub）、bcrypt | |
| 邮件 | 无 SDK，直接 HTTP 调 Cloudflare Email / Sendflare | `Sender` 接口双实现 + noop |
| 任务队列 | 进程内 channel（buffer 1024） | 无 Redis 无 cron；启动时从 DB 恢复未完成任务 |
| 服务端 i18n | 自研 `internal/i18n`，zh/en 双表 | 测试保证两表键一致 |

### 前端（`frontend/`）

| 层面 | 选型 | 备注 |
| --- | --- | --- |
| 框架 | React 19 + TypeScript 6 + Vite 8 | `build = tsc -b && vite build` |
| 样式 | Tailwind CSS v4（CSS-first） | 无 config 文件，`@theme` 块写在 `index.css` |
| 路由 | react-router-dom 7 | 扁平路由；`/:code` 是短链分享页，兜底是首页 |
| 图表 | chart.js + react-chartjs-2 | 配套 `chartTheme.ts` |
| Passkey | @simplewebauthn/browser | |
| i18n | 自研 `src/i18n/`（zh.ts 为事实来源，`Dict = typeof zh`） | **漏译直接编译失败**；插值用带类型的箭头函数 |
| 主题 | 深色固定，绿/紫双品牌色 | 见「开发习惯 · 设计」 |
| 灯箱 | LiteZoom（`public/static/litezoom.min.js?v=N`） | 全局 `<script>` 加载不进 bundle，类型在 `litezoom.d.ts` |

### macOS 客户端（[openimg-app](https://github.com/gentpan/openimg-app)）

| 层面 | 选型 | 备注 |
| --- | --- | --- |
| 语言 | Swift 6 + SwiftUI（+ AppKit 补面板/剪贴板） | **零第三方依赖**（刻意，CI 与非 Mac 贡献者可碰 Kit） |
| 结构 | SwiftPM 包，无 .xcodeproj | `OpenimgKit`（共用库，为 iOS 复用设计）+ `OpenimgMac`（应用）+ `KitCheck`（95 项自检） |
| 图表 | Swift Charts | 概览页配额/格式分布/签到 |
| 登录 | 密码换 365 天设备令牌 / OAuth（ASWebAuthenticationSession + `openimg://` 回调）/ 粘贴令牌 | 令牌存钥匙串 |
| 打包 | `./package-mac.sh [release]`（openimg-app 仓库） | 手写 Info.plist（io.openimg.mac），ad-hoc 签名；**禁止 `swift run OpenimgMac`**（裸可执行无 bundle 静默退出）；推 `v*` 标签由 CI 发布 |

### 基础设施

| 组件 | 角色 |
| --- | --- |
| PostgreSQL 18 | 唯一数据库 |
| MinIO（127.0.0.1:9000，桶 `openimg`） | 平台存储池；Host 头须匹配 `MINIO_SERVER_URL` 走 path-style |
| Caddy（frankenphp 发行版） | 反代 + TLS + CDN 源站头 |
| Cloudflare | CDN、DNS、邮件发送；三个 zone：openimg.io / imgla.com / litezoom.dev |
| systemd | `openimg` 服务（仓库单元文件名 `openimg-backend.service`），NoNewPrivileges + ProtectSystem=strict |

### 域名布局

| 域名 | 角色 |
| --- | --- |
| openimg.io | 主站 + API + 短链（`/:code` 302）；Go 进程独占域名（`FRONTEND_DIR` 服务 SPA），生产端口 8090 |
| www.openimg.io | 301 → openimg.io |
| cdn.imgla.com | 原图直连（`rewrite * /openimg{uri}` → MinIO；根路径 301 回主站，防桶列表泄露） |
| cache.imgla.com | 缩略图直连；与原图同桶，分裂在域名层——缩略图可再生，单独 purge 便宜 |
| litezoom.dev | 灯箱库官网 + CDN（`/litezoom.min.js`） |

## 开发环境

### 依赖

| 组件 | 版本 | 说明 |
| --- | --- | --- |
| Go | 1.26+ | |
| Node | 24+ | |
| PostgreSQL | 17 或 18 | |
| libvips | 8.15+ | **cgo 依赖**，不能用 `CGO_ENABLED=0` 构建 |

```bash
# macOS
brew install go node postgresql@18 vips

# Debian / Ubuntu
apt install golang nodejs postgresql libvips-dev pkg-config
```

macOS 上如果 `go build` 报 `Package vips was not found in the pkg-config search path`：

```bash
export PKG_CONFIG_PATH="/opt/homebrew/lib/pkgconfig:$PKG_CONFIG_PATH"
```

### 数据库

```bash
createdb openimg
# 或者用 Docker
docker run -d --name openimg-pg \
  -e POSTGRES_USER=openimg -e POSTGRES_PASSWORD=openimg -e POSTGRES_DB=openimg \
  -p 5432:5432 postgres:18-alpine
```

表结构由 GORM 的 `AutoMigrate` 在启动时创建，无需手动跑 migration。用户组种子数据（admin / trusted / free）也在启动时写入。

### 配置

```bash
cd backend
cp .env.example .env
```

必填：`DATABASE_URL`、`JWT_SECRET`。

强烈建议填：

- `STORAGE_MASTER_KEY` — `openssl rand -base64 32`。不填则无法保存用户绑定的存储凭据，也无法在后台配置 OAuth
- `BOOTSTRAP_ADMIN_EMAIL` / `BOOTSTRAP_ADMIN_PASSWORD` — 首次启动创建管理员

开发环境可以把 `REQUIRE_EMAIL_VERIFIED` 设为 `false`，否则没配邮件服务时无法上传。
不配 `S3_BUCKET` 时会回退到本地磁盘（`STORAGE_DIR`），整套流程照样能跑通。

配置面全貌见 `backend/.env.example`（Core / Auth / Encryption / 平台存储池 / 上传流水线 / OAuth / Email / Bootstrap 八组，每个键带注释）。注意 S3 那组只在**首次启动**引导平台 StorageProfile 行，之后以数据库为准。

### 运行

```bash
# 后端 :8080
cd backend && go run .

# 前端 :5173，vite 代理 /api /auth /admin/api /storage /healthz → :8080
cd frontend && npm install && npm run dev

# macOS 客户端(独立仓库 openimg-app)
cd ../openimg-app/OpenimgKit && swift run KitCheck   # 95 项自检
../package-mac.sh                                    # 打 .app(勿 swift run OpenimgMac)
```

### 测试与检查

```bash
cd backend
go build ./... && go vet ./... && go test ./...

cd frontend
npx tsc -b && npm run build && npx eslint .
```

`internal/imageproc` 有单元测试，覆盖格式嗅探、SVG 拒绝、变体生成、不放大、超尺寸拒绝、EXIF 剥离。
`internal/i18n` 的测试保证 zh/en 两表键一致。App 侧 `KitCheck` 是自检可执行目标而非 XCTest——只装 CLT 时 `swift test` 编译不了。

### 常见问题

| 现象 | 原因 |
| --- | --- |
| `Package vips was not found` | 没装 libvips，或 `PKG_CONFIG_PATH` 没设 |
| `required env var DATABASE_URL is empty` | `.env` 不在工作目录——后端先找 `/opt/openimg/config/.env` 再找 `./.env` |
| 上传报「存储不可用」 | 没配 S3 且 `STORAGE_DIR` 不可写 |
| 图片 URL 是相对路径 | `PUBLIC_BASE_URL` 没配 |
| 转换设置没生效 | 「上传自动转换」只对新处理的上传生效；同字节旧行若与新设置相容会走秒传克隆（不相容则重新处理） |
| 旧图的 AVIF 变体一直没生成 | 历史附加变体走后台队列：去重命中（不重新处理），或 AVIF 比原图大被丢弃；现在转换是上传时同步的 |
| 邮件发不出去 | 检查 provider 配置；异步发送不能用请求 context |
| 端口占用 | `lsof -ti:8080 \| xargs kill -9`（`pkill -f "go run"` 杀不掉子进程） |
| `swift run OpenimgMac` 无反应 | 故意的：裸可执行没有 bundle，用 `package-mac.sh` 打 .app |

## 开发习惯与约定

### 语言与文字

- **代码注释、提交信息、仓库文档全部中文**；面向国际用户的产物例外（litezoom.js 头注释是英文，因为它随 CDN 分发）
- 产品 UI 完整双语（zh/en），前端字典 `Dict = typeof zh` 让漏译成为编译错误；后端错误消息走 `internal/i18n`，按 Accept-Language 协商
- 注释写**为什么与不变式**，不写"这行做了什么"；坑记在离坑最近的地方（例：Caddy 剥头配置里写明它与 gin TrustedPlatform 是一对）

### 提交

- 中文提交信息：主题一行说清改了什么，正文讲**动机与机制**（为什么这么修、错在哪里）
- 不加任何共建者 / 工具署名行
- 主分支直接提交（单人项目），部署脚本内建校验兜底

### 设计

- **产品固定深色**（浅色主题已删除：品牌绿 #5DE31D 在白底对比度 1.79:1 不可用）
- 品牌色绿（#5DE31D）/ 紫（#8E47FF）一键切换：整套 `--color-brand-*` 变量在 `:root[data-brand="violet"]` 下覆盖，实心控件前景统一取 `--color-brand-ink`（绿配近黑、紫配白），切换零组件改动；favicon 用 data: URI 运行时重画
- 头像、旗帜、状态点、开关等"本身就该是正圆"的元素用 `rounded-full`，其余遵循页面既有圆角语言
- 品牌字体 Ubuntu 仅 Latin，CJK 落回系统栈；图标 Font Awesome
- 悬停动效把 transform 与 color 的过渡时长分开（共用一个时长会显得卡）

### 成对配置（改一处必须同步另一处）

| 一侧 | 另一侧 | 断了会怎样 |
| --- | --- | --- |
| gin `TrustedPlatform`（CF-Connecting-IP） | Caddy `@not_cloudflare` 剥头 | 直连源站可伪造任意 IP；或限流按 CF 边缘 IP 分桶 |
| `models.ReservedShortCodes` 保留词表 | 前端路由路径 | 短码遮蔽 /login 这类页面 |
| Caddy `header_up Host` | MinIO `MINIO_SERVER_URL` | path-style 解析失效，主机名被当桶名 |
| App `X-Openimg-Brand` 头 | 后端 CORS AllowHeaders + 邮件渲染 | OTP 邮件主题色对不上 |

### 安全

- 凭据只进 `.env` / `deploy/.env.local`（0600、gitignored）；部署脚本第 2 步扫描 Google/AWS 密钥形态，命中即中止
- API Token、OTP、会话 raw token 一律**只存哈希**；OTP 按 purpose 隔离防跨用途钓鱼
- 上传一律解码重编码杀 polyglot，SVG/PDF 拒收；「原样存储」模式也要零填 GPS EXIF
- 对象存储侧 `nosniff` + `CSP sandbox`；未知扩展名一律 octet-stream
- 秒传去重命中配额照扣——省的是平台成本，不是用户空间（防跨用户探测）

### 验证

- **部署即验证**：deploy.sh 六步里三步是校验（本地 build/vet/test/tsc、凭据扫描、上线后 Content-Type 断言）
- 改 UI 后在浏览器里做 DOM 级实测（计算样式、几何对齐、交互路径），不满足于"代码看起来对"
- 服务器状态类改动先备份原文件、配置校验通过才 reload，失败自动回滚
- 验证要看 Content-Type 不只看状态码——被 SPA 兜底吞掉的 /assets/*.js 会 200 返回 HTML，页面白屏

### CDN 缓存策略

- 对象 key 写入后永不复用 → 图片响应 `max-age=31536000, immutable` 一年安全
- 页面/静态资源改动后 purge 对应 zone；**只改个别文件时精准 purge 单 URL**，不动整站图片缓存
- 前端引用的共享库带 `?v=N` 版本参数，改库时递增

### LiteZoom 同步链

源头在 [gentpan/litezoom](https://github.com/gentpan/litezoom) 仓库 → 部署到 litezoom.dev（源码 + min 都在站根）：

- **litenote.io**：直接引 CDN `https://litezoom.dev/litezoom.min.js`，改库自动生效
- **openimg**：持有本地副本 `frontend/public/static/litezoom{,.min}.js`，改库后手动拷贝 + `index.html` 里 `?v=` 递增 + 重新部署

## 部署

### 生产服务器

| 服务器 | 角色 | 路径 |
| --- | --- | --- |
| root@88.198.27.78 | openimg 生产（后端 + 前端 + MinIO + Caddy/frankenphp） | `/opt/openimg`，Caddy 配置 `/etc/frankenphp/sites/openimg.caddy`（仓库副本 `deploy/caddy/openimg.caddy`，改动两边同步） |
| root@88.198.40.218 | litezoom.dev 静态站 | `/opt/litezoom/site`，Caddy 配置 `/etc/caddy/Caddyfile` |

SSH 私钥默认 `~/Desktop/gentpan.pem`（可用 `OPENIMG_HOST` / `OPENIMG_KEY` 覆盖）。

### 一键部署

```bash
./deploy/deploy.sh              # 部署
./deploy/deploy.sh --rollback   # 回滚到上一个二进制
```

六步流程，任一步失败即中止：

1. **本地校验** — 后端 build/vet/test，前端 tsc -b + build
2. **凭据扫描** — grep Google OAuth secret / AWS key 形态，命中即中止
3. **上传** — 先在远端把当前二进制备份为 `.prev`；rsync 后端源码与前端 dist
4. **远端编译并切换** — 编到 `.new` 成功才 mv 替换，`systemctl restart openimg`；5 秒后不 active 自动回滚 `.prev`
5. **清 CDN 缓存** — 凭据在 `deploy/.env.local`（`CF_API_EMAIL` + `CF_API_KEY`；清空 EMAIL 即切换为 scoped token 模式）。purge 失败只警告不中止，但必须出声
6. **验证** — 三个域名 200、www 301、**逐个断言 /assets/* 的 Content-Type**、API 响应含 total_images

回滚只有一代（`.prev`），每次部署会覆盖。

### 手动部署（备忘）

```bash
# 服务器上
git pull
cd backend && go build -o /opt/openimg/bin/openimg-server .
systemctl restart openimg
cd frontend && npm ci && npm run build   # dist/ 由 Go 进程服务（FRONTEND_DIR）
```

Docker 见 `backend/Dockerfile` 与根目录 `docker-compose.yml`（postgres + backend + frontend 三服务，本地/自托管向）。镜像基于 Debian 而非 Alpine——libvips 在 musl 上历史性缺 AVIF/HEIC 编解码器。

### litezoom.dev 部署

无脚本，scp 即部署：改动文件 scp 到 `root@88.198.40.218:/opt/litezoom/site/`，然后清 Cloudflare litezoom.dev zone。
源站证书是自签名（Cloudflare Full 模式不验 CA），访客侧证书是 CF Universal SSL。

### 注意事项

- **`STORAGE_MASTER_KEY` 与数据库分开备份。** 丢了它，所有用户绑定的存储桶凭据都解不开
- **`.env` 不要提交。** 已在 `.gitignore` 中
- 生产环境务必 `COOKIE_SECURE=true`
- `TEMP_DIR` 放 SSD——转码临时文件读写频繁
- 服务器上 known_hosts 偶发丢条目，`ssh-keyscan` 补回即可（BatchMode 下表现为 Host key verification failed）

## macOS / iOS 客户端

客户端在独立仓库 [gentpan/openimg-app](https://github.com/gentpan/openimg-app)（SwiftPM 包，无 .xcodeproj），三个 target：

- **OpenimgKit** — 共用库（API 客户端、模型、钥匙串、multipart、上传前本地降宽、网格几何），为 Mac/iOS 两端复用设计，`.iOS(.v17)` 已声明
- **OpenimgMac** — SwiftUI 窗口应用（约 3800 行）：四页（概览/图库/上传/设置）+ 全窗灯箱 + 登录。状态集中在单个 `@MainActor` AppModel（视图不用 @State：CLT 无法展开宏）
- **KitCheck** — 56 项自检可执行（替代 XCTest：只装 CLT 时 swift test 编译不了）

与后端的集成点：

- 登录换**365 天设备令牌**（`oimg_` API token），存钥匙串；OAuth 走 `openimg://` 私有 scheme + 60 秒单次 code 换令牌（`/auth/native/exchange`）
- 每个请求带 `Authorization: Bearer` 和 `X-Openimg-Brand`（服务器按调用方主题渲染 OTP 邮件）
- 令牌可达"呈现类"接口（上传/图库/配额/签到/偏好/头像）；铸 token 和删号是 cookie-only——防令牌泄露升级为账号接管
- CDN 图片请求刻意不带令牌，防泄露给第三方存储

当前状态：`swift build` 通过、KitCheck 95/95、GitHub Releases 提供打包产物、源码零 TODO。
未竟：iOS 端未动工（阻塞在产品决策：App Store 5.1.1(v) 要求 app 内可删账号，而删号是 cookie-only）；
分发给他人需要 Developer ID 签名 + 公证（未做）；passkey 原生 API 与数据保护钥匙串需要付费开发者证书（ad-hoc 下自动回退网页/旧式钥匙串，可用）。

## 进度

### 时间线（107+ 提交，2026-08-05 起）

前身项目 picbi 至少 2026-05 已有生产数据（`server-backup-20260510/`），8 月初以成品形态重建 git 史。

| 里程碑 | 日期 | 内容 |
| --- | --- | --- |
| M0 初始骨架 | 08-05 | 单笔 squash（152 文件 / 2.3 万行）：数据模型、上传流水线、配额账本、签到、BYOS、邀请、Passkey、管理后台一次到位 |
| M1 上线冲刺 | 08-05 | 短链 + 落地页 + 表情反应、举报体系、缩略图独立域名、注册邮箱验证码、EXIF 剥 GPS、生产部署 |
| M2 macOS 客户端 | 08-05 | 客户端从骨架到可用（后拆分为 openimg-app 仓库）：四页 + 灯箱 + 三种登录 + 打包脚本（约 20 笔） |
| M3 品牌视觉定稿 | 08-05→06 | 紫→绿→双色系→最终「深色固定 + 绿/紫可切」；删浅色主题，ThemeContext 收敛为 BrandContext |
| M4 打磨与清理 | 08-06 | /docs 文档页、部署脚本自动清 CDN、favicon 双品牌 + 内容指纹、三轮死代码清理 |
| M5 i18n 双语 | 08-07 | 前后端全量 zh/en（前端字典 ~1800 行、后端 160 处消息），docs 双语 |
| M6 admin 强化 | 08-07 | 空间流水分页/搜索（修 GORM 命名参数混用错位 + LIKE 转义）、全站样式化对话框、缩略图策略可配、admin 静态子路由、用户组徽章、修限流误按 CF 边缘 IP 分桶 |
| M7 近期迭代 | 08-09→12 | 签到累计空间展示、OTP 邮件随主题双语、首页 SEO 文案、LiteZoom 灯箱接入与四轮同步、圆形旗帜语言切换、登录/注册合并单按钮、CDN 根路径 301 |

### 当前状态

- 生产运行于 openimg.io，前后端 + Mac App + 灯箱库全部上线
- 代码零 TODO/FIXME 遗留；部署脚本、回滚、监控头（healthz/public-stats）齐备
- LiteZoom 独立成库（MIT），官网 litezoom.dev 双语 + GitHub 星标 + 自演示

### 已知未竟事项

| 事项 | 阻塞点 |
| --- | --- |
| iOS 客户端 | 产品决策：删号 cookie-only vs App Store 5.1.1(v)；Kit 层已就绪 |
| Mac App 对外分发 | Developer ID 签名 + 公证未做（本机 ad-hoc 可用） |
| `.env.example` 的 `S3_PUBLIC_URL_BASE` 示例值 | 写的还是 cdn.openimg.io，生产实际是 cdn.imgla.com（示例性质，自托管者会改） |
| deploy/systemd 单元文件名 | 仓库叫 `openimg-backend.service`，生产服务名是 `openimg`，装机时注意 |
