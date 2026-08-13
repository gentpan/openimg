# Openimg Architecture

免费图床。上传经过后端做压缩转换，读取直连 CDN —— 服务器只承担上传方向的带宽。

```
                    ┌──────────────────────────────────────┐
   上传 ──────────▶ │  Go 后端（唯一必经之路）              │
   (必经后端)        │  MIME校验 → SHA256 → vips压缩/转码    │
                    │  → EXIF剥离 → 多规格生成 → 存储路由    │
                    └───────────────┬──────────────────────┘
                                    │ 并发写
                 ┌──────────────────┼──────────────────┐
                 ▼                  ▼                  ▼
        ┌────────────────┐  ┌──────────────┐  ┌──────────────┐
        │ 平台存储池      │  │ 用户自己的桶  │  │ 备份目标      │
        │ (MinIO/配额)   │  │ (BYOS/无配额) │  │ (异步队列)    │
        └───────┬────────┘  └──────┬───────┘  └──────────────┘
                │                  │
                ▼                  ▼
   cdn/cache.imgla.com    用户自己的域名
                └── Cloudflare CDN ──┘
                          │
                       读取直连 CDN，不过后端
```

## 后端（`backend/`，Go 1.26 + Gin + GORM）

`main.go` 依次装配：config → DB → 账本回填 → crypto → storage registry → auth → imageproc → 队列 → HTTP 路由。收到 SIGINT/SIGTERM 优雅退出。

### `internal/` 模块

| 模块 | 职责 |
| --- | --- |
| `api` | Gin 路由与 handler。上传、图库、配额、存储配置、管理后台、后台任务处理器 |
| `auth` | JWT + Cookie 会话、OAuth、API Token 校验、管理员守卫 |
| `passkey` | WebAuthn 注册与登录。RP ID = `openimg.io` |
| `crypto` | AES-256-GCM，加密用户提交的存储桶凭据 |
| `imageproc` | libvips 封装：解码、剥离元数据、限宽、重编码、生成变体 |
| `storage` | 多租户 S3 抽象 + Registry（按 profile 解析并缓存客户端）+ SSRF 防护 |
| `quota` | 字节账本：发放 / 占用 / 释放 / 对账 / 回填 |
| `checkin` | 每日签到，随机发放，唯一索引保证幂等 |
| `scheduler` | 通用任务队列：AVIF 转码、备份复制、对象清理 |
| `referral` | 邀请码生成与双向奖励 |
| `email` | `Sender` 接口 + Cloudflare / Sendflare 两个实现 |
| `models` | GORM 实体 |
| `config` | godotenv 加载 `/opt/openimg/config/.env` 然后 `./.env` |
| `db` | PostgreSQL bootstrap、AutoMigrate、用户组种子数据 |

### 上传流水线

1. `POST /api/upload`，`MaxBytesReader` 按用户组限流
2. 校验：邮箱已验证、账号状态、当日上传次数
3. `TeeReader` 边读边算 SHA-256
4. **真实 MIME 嗅探**（不信扩展名、不信 Content-Type），拒绝 SVG
5. **去重**：查 `(profile_id, sha256, proc_sig)`——同字节且处理设置（模式/转换目标/限宽）一致才克隆元数据；设置不同则按当前设置重新处理，设置生效优先于去重省空间
6. `imageproc.Process`：自动旋转 → 限宽 → 剥离 EXIF → 重编码（选了 WebP/AVIF 则主图直接编码为目标格式，不保留原格式；动图不转换）→ 网格缩略图
7. 配额预扣（写对象之前）
8. 并发写入目标存储，任一变体失败则回滚已写对象并退还配额
9. 写 DB，备份复制入队异步处理（AVIF 是同步转换的，无异步补变体路径）

### 关键设计

**URL 不入库。** 只存相对 `object_key`，公开地址在读取时由 `profile.PublicBaseURL` 拼接。换 CDN 域名不会让历史图片全废。

**按日期分目录 + 随机 ID。** `2026/08/04/aB3dEf7hJ9kL.jpg`，日期取 UTC 上传日。ID 为 12 位
`0-9a-zA-Z`（62 字符表，拒绝采样避免模偏），不含 `-`/`_`——它们能过 URL，但过不了双击选中、
邮件折行和 Markdown 强调。写入后永不覆盖，所以一年 immutable 缓存依然成立。

去重不依赖 key 的算法：命中 `(profile_id, sha256)` 且处理签名（`proc_sig`）一致才复用 twin
的 `object_key`，因此换 key 格式不影响去重；同字节可能因设置不同各存一份对象。变体 key 由**已存的 `object_key`** 推导而非从 sha 重算，
旧的 `img/ab/cd/<sha256>.jpg` 推出的变体路径与历史完全一致，两种布局共存、无需迁移。

**删除前检查引用计数。** 去重让多个用户共享同一对象，`jobPurge` 按 `(profile_id, object_key)`
确认无其他活跃引用才真正删除——同 sha 不同格式的行各自持有各自的对象，互不挡清理。

**短链在根路径。** `openimg.io/aB3d`，4–6 位 `0-9a-zA-Z`，302 跳到图片。代码空间与前端路由重叠
（`login`/`admin`/`space`/`refer` 都是 5 字符，`upload` 是 6），所以有一张保留词表：生成时跳过、
查找时拒绝，两边用同一份数据因而不会失配。去重产生的副本各自持有独立短链——共享对象，但删掉一条
不该弄坏另一条的链接。

**账本可重放。** `QuotaBytes` / `UsedBytes` 是冗余计数器，`quota_transactions` 是权威流水，可重放校验，启动时自动回填缺失记录。

**BYOS 不占平台配额。** 用户自己的桶自己付费，只约束处理次数。

### 路由分组

| 前缀 | 认证 | 说明 |
| --- | --- | --- |
| `/healthz`、`/api/public-stats` | 无 | 公开 |
| `/api/report` | 可选 | 匿名可举报，独立限流 |
| `/auth/...` | 混合 | 注册、登录、OAuth、邮箱验证码、Passkey |
| `/api/upload`、`/api/images` | Cookie **或** API Token | 机器可访问 |
| `/api/quota`、`/api/checkin`、`/api/storage/profiles`、`/api/tokens`、`/api/account` | 仅 Cookie | Token 不能改账号设置 |
| `/admin/api/...` | 管理员 | 统计、用户、用户组、图片、举报、流水、OAuth 配置 |

### 数据模型

- **User** — 邮箱、密码哈希、角色、用户组、OAuth 绑定、`QuotaBytes`/`UsedBytes`、签到状态、转换偏好、注册/登录 IP
- **UserGroup** — 单文件上限、每日张数、允许格式、注册赠送、签到区间、连续奖励、邀请奖励、空间上限、BYOS 开关
- **Image** — `sha256`、`object_key`、尺寸、原始/存储大小、变体 CSV、备份状态、软删除
- **StorageProfile** — 平台或用户的 S3 配置，凭据加密，可设为默认或备份目标
- **QuotaTransaction** — 容量与占用两族流水，双计数器快照
- **CheckinRecord** — `(user_id, date)` 唯一索引保证幂等
- **APIToken** — 仅存 SHA-256，明文只显示一次
- **Report** — 举报与处理结果
- **Album**、**EmailOTP**、**PasskeyCredential**、**SiteSetting**、**Session**

## 前端（`frontend/`，React 19 + Vite + TS + Tailwind v4）

| 路径 | 页面 |
| --- | --- |
| `/` | 首页：上传框即主视觉，滚动入场动画 |
| `/dashboard` | 用户总览：空间条、趋势、格式分布、压缩效果、签到日历 |
| `/upload` | 上传页 |
| `/gallery` | 图库：搜索、批量删除、详情侧栏 |
| `/space` | 空间：配额、签到、流水 |
| `/settings` | 存储配置、API Token、压缩转换偏好、登录方式、删除账户 |
| `/refer` | 邀请 |
| `/admin/*` | 管理后台（六个 Tab） |

- 上传队列提到全局 Context，悬浮在右下角，切换页面不中断
- 所有页面容器统一 `max-w-7xl`，`scrollbar-gutter: stable` 防止切页抖动
- 品牌字体 Ubuntu，主色 `#8E47FF`

## 部署

`deploy/systemd/openimg-backend.service` — 以 `openimg` 用户运行 `/opt/openimg/bin/openimg-server`，已加固（NoNewPrivileges、ProtectSystem=strict）。反向代理用 Caddy。

Docker 镜像基于 Debian 而非 Alpine：libvips 在 musl 上历史性缺少 AVIF/HEIC 编解码器。
