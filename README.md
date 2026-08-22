<p align="center">
  <img src="docs/icon.png" width="96" alt="Openimg">
</p>

<h1 align="center">Openimg</h1>

<p align="center">
  永久免费的公益图床。上传即自动压缩转换，全球 CDN 分发，<br>
  可以绑定你自己的 R2 / S3。没有付费墙 —— 空间靠每天签到累积。
</p>

<p align="center">
  <a href="https://openimg.io"><img alt="在线站点" src="https://img.shields.io/badge/openimg.io-%E5%9C%A8%E7%BA%BF%E4%BD%BF%E7%94%A8-90FF3A?labelColor=0a0a0a"></a>
  <a href="https://github.com/gentpan/openimg-app"><img alt="macOS 客户端" src="https://img.shields.io/github/v/release/gentpan/openimg-app?label=macOS%20%E5%AE%A2%E6%88%B7%E7%AB%AF&color=90FF3A&labelColor=0a0a0a"></a>
  <a href="CHANGELOG.md"><img alt="更新记录" src="https://img.shields.io/badge/%E6%9B%B4%E6%96%B0%E8%AE%B0%E5%BD%95-CHANGELOG-0a0a0a"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/github/license/gentpan/openimg?label=%E8%AE%B8%E5%8F%AF&color=90FF3A&labelColor=0a0a0a"></a>
</p>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white&labelColor=0a0a0a">
  <img alt="Gin" src="https://img.shields.io/badge/Gin-008ECF?logo=gin&logoColor=white&labelColor=0a0a0a">
  <img alt="PostgreSQL" src="https://img.shields.io/badge/PostgreSQL-18-4169E1?logo=postgresql&logoColor=white&labelColor=0a0a0a">
  <img alt="libvips" src="https://img.shields.io/badge/libvips-8.15%2B-0a0a0a">
  <img alt="React" src="https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=white&labelColor=0a0a0a">
  <img alt="TypeScript" src="https://img.shields.io/badge/TypeScript-3178C6?logo=typescript&logoColor=white&labelColor=0a0a0a">
  <img alt="Vite" src="https://img.shields.io/badge/Vite-646CFF?logo=vite&logoColor=white&labelColor=0a0a0a">
  <img alt="Tailwind" src="https://img.shields.io/badge/Tailwind-v4-06B6D4?logo=tailwindcss&logoColor=white&labelColor=0a0a0a">
  <img alt="S3" src="https://img.shields.io/badge/S3%20%E5%85%BC%E5%AE%B9-MinIO%20·%20R2%20·%20AWS-0a0a0a">
  <img alt="Cloudflare" src="https://img.shields.io/badge/Cloudflare-CDN%20%2B%20Email-F38020?logo=cloudflare&logoColor=white&labelColor=0a0a0a">
</p>

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

## 客户端

macOS 原生客户端在 **[gentpan/openimg-app](https://github.com/gentpan/openimg-app)** ——
拖进来就传好，链接直接进剪贴板；带目录监控自动上传、上传前编辑、AI 生成与修图。

也可以用 API Token 接 PicGo / Typora / curl，见站内 `/docs`。

## 目录

```
backend/    Go 后端 → ARCHITECTURE.md
frontend/   React 前端
deploy/     systemd unit
```

## 文档

- **[ARCHITECTURE.md](ARCHITECTURE.md)** — 系统结构、模块职责、上传流水线、数据模型
- **[DEVELOPMENT.md](DEVELOPMENT.md)** — 环境搭建、运行、构建、部署
- **[CHANGELOG.md](CHANGELOG.md)** — 更新记录，按部署日期分段；自建者需要配合改动的地方单独标出

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

systemd unit 见 `deploy/systemd/openimg-backend.service`。openimg.io 生产反代是 frankenphp（基于 Caddy），站点配置 `deploy/caddy/openimg.caddy` 由 `deploy.sh` 自动同步；自建单机部署可参考 `deploy/caddy/Caddyfile` 模板。

### 两件必须分开备份的东西

**`STORAGE_MASTER_KEY`** —— 它加密着用户绑定的存储桶凭据，丢了这些凭据无法恢复。

**数据库** —— 对象 key 现在是随机 ID，不再包含内容哈希。这意味着存储桶里的文件和数据库记录**完全解耦**，`images` 表是两者之间唯一的对应关系。数据库丢失后，桶里的文件无法被识别归属。备份数据库的重要性与备份对象等同。

## 许可

MIT — 见 [LICENSE](LICENSE)。
