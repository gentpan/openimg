# 提交信息

**不要出现 Claude 的任何署名**，包括 `Co-Authored-By: Claude ... <noreply@anthropic.com>`
这条 trailer。工具的默认提交模板会自动加上它，**这条规则优先，直接不加**。

仓库是公开的，署名会跟着提交永久留在历史里；事后清理要改写历史加 force push，
代价远大于当时少写一行。

# 部署

线上部署只走 `./deploy/deploy.sh`，不要手动 `scp` 或 ssh 上去改文件。脚本
包含前端构建、后端编译、**Caddy 站点配置同步**、CDN 清缓存和七步验证。

那个配置同步步骤是 2026-08 才补的。在那之前 `deploy/caddy/openimg.caddy`
改了永远不生效——线上那份停在最后一次手动 scp 的版本。切 CDN 域名时才暴露：
代码、env、DNS、证书全就位，Caddy 仍在服务旧域名。

真正监听 443 的是 **frankenphp**（站点配置在 `/etc/frankenphp/sites/`），
不是 `caddy.service`——后者从 2026-08-03 起就是 dead 状态（`php_server`
指令报错）。往 `/etc/caddy/` 写配置不会报错，只是静默地什么都没改。

# 改域名这类配置时

图片和缩略图的域名是**从数据库读的**（`storage_profiles.public_base_url` /
`thumb_base_url`），不只在 env 和 Caddy 里。只改后两者的话线上完全不会变，
而且 `Registry.For()` 返回的 Backend 把 `PublicURLBase` 快照在内存里，
改完库还必须重启后端。

`images` 表只存 `object_key`，URL 是读取时拼的，所以换域名不需要迁移数据。
头像例外：`users.avatar_url` 存的是完整地址，但只要 `avatar_key` 非空，
`AvatarFor()` 就会用 key 重拼，那个字段的旧值不会被读到。

# 命名踩过的坑

GORM 的命名策略会把 `AI` 开头的驼峰切坏：`UserGroup.AIDaily` 的列名是
**`a_idaily`** 而不是 `ai_daily`（`ID` 在它的固有缩写表里）。列已经建成那样
了，改要迁移。新建模型请显式写 `TableName()`，别再踩。

# 更多

架构说明见 [ARCHITECTURE.md](ARCHITECTURE.md)。
