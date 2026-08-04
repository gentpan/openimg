# 开发指南

## 依赖

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

## 数据库

```bash
createdb openimg
# 或者用 Docker
docker run -d --name openimg-pg \
  -e POSTGRES_USER=openimg -e POSTGRES_PASSWORD=openimg -e POSTGRES_DB=openimg \
  -p 5432:5432 postgres:18-alpine
```

表结构由 GORM 的 `AutoMigrate` 在启动时创建，无需手动跑 migration。用户组种子数据（admin / trusted / free）也在启动时写入。

## 配置

```bash
cd backend
cp .env.example .env
```

必填：

- `DATABASE_URL`
- `JWT_SECRET`

强烈建议填：

- `STORAGE_MASTER_KEY` — `openssl rand -base64 32`。不填则无法保存用户绑定的存储凭据，也无法在后台配置 OAuth
- `BOOTSTRAP_ADMIN_EMAIL` / `BOOTSTRAP_ADMIN_PASSWORD` — 首次启动创建管理员

开发环境可以把 `REQUIRE_EMAIL_VERIFIED` 设为 `false`，否则没配邮件服务时无法上传。

不配 `S3_BUCKET` 时会回退到本地磁盘（`STORAGE_DIR`），整套流程照样能跑通。

## 运行

```bash
# 后端 :8080
cd backend && go run .

# 前端 :5173，代理 /api /auth /admin/api /storage → :8080
cd frontend && npm install && npm run dev
```

## 构建

```bash
cd backend && go build -o openimg-server .     # 需要 libvips
cd frontend && npm run build                   # 产物在 dist/
```

## 测试与检查

```bash
cd backend
go build ./... && go vet ./... && go test ./...

cd frontend
npx tsc -b        # 类型检查
npm run build
npx eslint .
```

`internal/imageproc` 有单元测试，覆盖格式嗅探、SVG 拒绝、变体生成、不放大、超尺寸拒绝、EXIF 剥离。

## 常见问题

| 现象 | 原因 |
| --- | --- |
| `Package vips was not found` | 没装 libvips，或 `PKG_CONFIG_PATH` 没设 |
| `required env var DATABASE_URL is empty` | `.env` 不在工作目录 —— 后端先找 `/opt/openimg/config/.env` 再找 `./.env` |
| 上传报「存储不可用」 | 没配 S3 且 `STORAGE_DIR` 不可写 |
| 图片 URL 是相对路径 | `PUBLIC_BASE_URL` 没配 |
| 后台任务全部 `profile not found` | 已修复；若复现说明 `ForStored` 的 nil 分支被绕过了 |
| AVIF 一直没生成 | 该图是去重命中（不重新处理），或用户关闭了 AVIF，或 AVIF 比原图大被丢弃 |
| 邮件发不出去 | 检查 provider 配置；异步发送不能用请求 context |
| 端口占用 | `lsof -ti:8080 \| xargs kill -9`（`pkill -f "go run"` 杀不掉子进程） |

## 部署

```bash
# 服务器上
git pull
cd backend && go build -o /opt/openimg/bin/openimg-server .
systemctl restart openimg-backend

cd frontend && npm ci && npm run build   # dist/ 交给 Caddy
```

Docker 见 `backend/Dockerfile`（基于 Debian，因为 Alpine 的 libvips 常缺 AVIF/HEIC 编解码器）。

## 注意事项

- **`STORAGE_MASTER_KEY` 与数据库分开备份。** 丢了它，所有用户绑定的存储桶凭据都解不开
- **`.env` 不要提交。** 已在 `.gitignore` 中
- 生产环境务必 `COOKIE_SECURE=true`
- `TEMP_DIR` 放 SSD —— 转码临时文件读写频繁
