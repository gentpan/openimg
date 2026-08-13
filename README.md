# Openimg

永久免费的公益图床 · **[openimg.io](https://openimg.io)**

上传即自动压缩转换，全球 CDN 分发，支持绑定用户自己的 R2 / S3 存储。
没有付费墙 —— 存储空间靠每天签到累积。

## 能力

- **上传流水线**：真实 MIME 嗅探 → SHA-256 去重 → EXIF/GPS 剥离 → 强制重编码 → 按需限宽 → WebP / AVIF
- **按需缩略图**：上传只生成 200px 网格图；600 / 1200px 在用户点击时才生成，不点不占空间
- **空间经济**：注册赠送、每日随机签到、连续签到奖励、邀请奖励，全部走可重放的字节账本
- **自有存储 (BYOS)**：用户可绑定自己的 R2 / S3 / MinIO，凭据 AES-256-GCM 加密存储，保存前跑写入探针
- **双品牌色**：产品固定深色，绿 / 紫两套品牌色一键切换——所有实心控件的前景取自单个 CSS 变量，切换零组件改动
- **多端接入**：API Token 兼容 PicGo / Typora / curl
- **管理后台**：总览图表、用户与用户组、图片审核、举报处理、空间流水、图片域名、上传策略

## 几个值得知道的设计

**图片域名与站点域名分离。** 对象 key 不含主机名，公开地址在读取时由存储档案的 `PublicBaseURL` 拼出。换 CDN 域名不会让历史图片全废，图片请求也不会带上 session cookie。

**对象 key 是 `2026/08/05/aB3dEf7hJ9kL.jpg`。** 日期取 UTC 上传日，ID 为 12 位 `0-9a-zA-Z`（拒绝采样避免模偏）。去重不依赖 key 的算法——命中 `(profile_id, sha256)` 且**处理签名一致**（模式 / 转换目标 / 限宽相同）才复用已有 key，设置不同则按新设置重新处理；引用计数按 `object_key` 统计，所以 key 格式可以改。变体路径由已存的 key 推导而非从 sha 重算，因此历史上的内容寻址布局无需迁移即可共存。

**账本是权威，计数器是缓存。** `quota_transactions` 可重放校验，`QuotaBytes` / `UsedBytes` 只是冗余计数器，启动时自动回填缺失记录。

**删除走引用计数。** 去重让多个用户共享同一对象，purge 任务按 `(profile_id, object_key)` 确认无其他活跃引用才真正删除。

**敏感操作需邮箱验证码。** 改密码、添加 Passkey、清空图库都要过验证码，且验证码**按用途绑定哈希**——为改密码发的码无法用于登录。

## 技术栈

| 层 | 选型 |
| --- | --- |
| 后端 | Go 1.26 + Gin + GORM |
| 数据库 | PostgreSQL 18 |
| 图片处理 | libvips（govips，cgo） |
| 存储 | S3 兼容（MinIO / R2 / AWS 均可） |
| 前端 | React 19 + Vite + TypeScript + Tailwind v4 |
| 邮件 | Cloudflare Email Sending |
| CDN | Cloudflare |

## 目录

```
backend/    Go 后端 → ARCHITECTURE.md
frontend/   React 前端
deploy/     systemd unit
```

## 文档

- **[ARCHITECTURE.md](ARCHITECTURE.md)** — 系统结构、模块职责、上传流水线、数据模型
- **[DEVELOPMENT.md](DEVELOPMENT.md)** — 环境搭建、运行、构建、部署

## 快速开始

需要 Go 1.26、Node 24、PostgreSQL 18、libvips 8.15+。

libvips 是 cgo 依赖，**不能用 `CGO_ENABLED=0` 构建**；Docker 镜像也不要用 Alpine —— musl 下的 AVIF / HEIC 编解码器不全。

```bash
brew install vips                      # macOS
apt install libvips-dev pkg-config     # Debian / Ubuntu
```

```bash
# 后端
cd backend
cp .env.example .env                   # 至少填 DATABASE_URL 和 JWT_SECRET
go run .

# 前端（另开终端，/api 代理到 :8080）
cd frontend
npm install
npm run dev                            # http://localhost:5173
```

## 生产环境

```
/opt/openimg/config/.env      配置
/opt/openimg/storage          本地存储（生产走 S3，此目录仅作开发回退）
/opt/openimg/tmp              转码临时文件（建议放 SSD）
/opt/openimg/bin/openimg-server
```

systemd unit 见 `deploy/systemd/openimg-backend.service`，反向代理用 Caddy。

### 两件必须分开备份的东西

**`STORAGE_MASTER_KEY`** —— 它加密着用户绑定的存储桶凭据，丢了这些凭据无法恢复。

**数据库** —— 对象 key 现在是随机 ID，不再包含内容哈希。这意味着存储桶里的文件和数据库记录**完全解耦**，`images` 表是两者之间唯一的对应关系。数据库丢失后，桶里的文件无法被识别归属。备份数据库的重要性与备份对象等同。

## 许可

MIT — 见 [LICENSE](LICENSE)。
