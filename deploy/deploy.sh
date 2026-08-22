#!/usr/bin/env bash
#
# Deploy openimg to production.
#
#   ./deploy/deploy.sh            build, upload, restart, verify
#   ./deploy/deploy.sh --rollback restore the previous binary
#
# Two things this script exists to prevent, both of which happened by hand:
#
#   1. Verifying with status codes alone. A misrouted /assets/index-abc.js
#      answers 200 with an HTML body; the browser gets HTML where it expected
#      JavaScript and the page renders blank. Only Content-Type catches that,
#      so the checks below assert on it.
#   2. Replacing the running binary before the new one compiles. The build
#      goes to a temporary path and is moved into place only on success.
set -euo pipefail

HOST=${OPENIMG_HOST:-root@88.198.27.78}
KEY=${OPENIMG_KEY:-$HOME/Desktop/gentpan.pem}
REMOTE=/opt/openimg
SITE=https://openimg.io
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Local credentials, if present. Untracked and 0600 — see .gitignore. Sourced
# rather than exported by hand so a deploy never silently skips cache purging
# because someone forgot to set the variables in this particular shell.
[[ -f "$ROOT/deploy/.env.local" ]] && source "$ROOT/deploy/.env.local"

ssh_() { ssh -i "$KEY" -o BatchMode=yes "$HOST" "$@"; }
say()  { printf "\n\033[1m%s\033[0m\n" "$*"; }
ok()   { printf "  \033[32m✓\033[0m %s\n" "$*"; }
die()  { printf "  \033[31m✗\033[0m %s\n" "$*" >&2; exit 1; }

if [[ "${1:-}" == "--rollback" ]]; then
    say "回滚到上一个二进制"
    ssh_ 'test -f '"$REMOTE"'/bin/openimg-server.prev' || die "没有可回滚的备份"
    ssh_ 'mv '"$REMOTE"'/bin/openimg-server.prev '"$REMOTE"'/bin/openimg-server && systemctl restart openimg'
    sleep 4
    ssh_ 'systemctl is-active openimg' | grep -q active && ok "已回滚并重启" || die "回滚后服务未启动"
    exit 0
fi

say "1/7  本地校验"
cd "$ROOT/backend"
go build ./... >/dev/null           || die "后端编译失败"
go vet ./internal/... >/dev/null    || die "go vet 失败"
go test ./internal/... >/dev/null   || die "测试失败"
ok "后端 build / vet / test"
cd "$ROOT/frontend"
npx tsc -b --pretty false >/dev/null 2>&1 || die "TypeScript 校验失败"
npm run build >/dev/null 2>&1            || die "前端构建失败"
ok "前端 tsc / build"

say "2/7  凭据扫描"
cd "$ROOT"
if git diff --cached --quiet && git diff --quiet; then :; fi
if grep -rqE "GOCSPX-[A-Za-z0-9_-]{20,}|AKIA[0-9A-Z]{16}" \
        --include="*.go" --include="*.ts" --include="*.tsx" backend/ frontend/src/ 2>/dev/null; then
    die "源码中发现疑似凭据，已中止"
fi
ok "未发现硬编码凭据"

say "3/7  上传"
ssh_ "cp $REMOTE/bin/openimg-server $REMOTE/bin/openimg-server.prev"
ok "已备份当前二进制"
rsync -az --delete -e "ssh -i $KEY -o BatchMode=yes" \
    --exclude 'node_modules' --exclude '.git' --exclude '.env' \
    --exclude '/storage/' --exclude '/tmp/' \
    "$ROOT/backend/" "$HOST:$REMOTE/src-backend/"
ok "后端源码"
rsync -az --delete -e "ssh -i $KEY -o BatchMode=yes" \
    "$ROOT/frontend/dist/" "$HOST:$REMOTE/frontend/"
ok "前端产物"

# Caddy 站点配置也要同步。
#
# 这一步原来没有,后果是 deploy/caddy/openimg.caddy 改了也不生效——线上那份
# 一直停在最后一次手动 scp 的版本。切 CDN 域名时这个缺口才暴露:代码、env、
# DNS、证书全就位,而 Caddy 仍在服务旧域名,Cloudflare 拿到的是 525。
#
# 路径是 /etc/frankenphp/sites/。443 上跑的是 frankenphp;apt 的 caddy 服务
# 2026-08-03 起就停了,2026-08-22 已卸载并 mask。所有源站证书在
# /etc/frankenphp/certs/(同日从 /etc/caddy/certs 迁来,旧路径留了 symlink
# 兼容外部仓库的旧引用),配置里一律写新路径。
#
# validate 通过才 reload:配置有语法错时 reload 会让服务整个停掉,而那一刻
# 站点是活的。宁可不生效,也不能把活站点搞挂。
say "4/7  同步站点配置"
CADDY_REMOTE=/etc/frankenphp/sites/openimg.caddy
ssh_ "cp $CADDY_REMOTE $CADDY_REMOTE.bak-\$(date +%Y%m%d-%H%M%S)"
scp -i "$KEY" -o BatchMode=yes deploy/caddy/openimg.caddy "$HOST:$CADDY_REMOTE" >/dev/null
if ssh_ 'frankenphp validate --config /etc/frankenphp/Caddyfile' >/dev/null 2>&1; then
    ssh_ 'systemctl reload frankenphp' && ok "站点配置已生效"
else
    printf "  配置校验失败,已回滚\n" >&2
    ssh_ "cp \$(ls -t $CADDY_REMOTE.bak-* | head -1) $CADDY_REMOTE"
    die "Caddy 配置有误"
fi

say "5/7  远端编译并切换"
ssh_ bash -s <<REMOTE_SCRIPT
set -euo pipefail
export PATH=/usr/local/go/bin:\$PATH
export GOCACHE=$REMOTE/.gocache GOMODCACHE=$REMOTE/.gomod
cd $REMOTE/src-backend
CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o $REMOTE/bin/openimg-server.new .
mv $REMOTE/bin/openimg-server.new $REMOTE/bin/openimg-server
systemctl restart openimg
REMOTE_SCRIPT
sleep 5
ssh_ 'systemctl is-active openimg' | grep -q active || {
    printf "  服务未启动，自动回滚…\n" >&2
    ssh_ "mv $REMOTE/bin/openimg-server.prev $REMOTE/bin/openimg-server && systemctl restart openimg"
    die "部署失败，已回滚"
}
ok "编译并重启完成"

say "6/7  清 CDN 缓存"
if [[ -n "${CF_API_KEY:-}" ]]; then
    # Two credential shapes. A Global API Key needs the account email alongside
    # it; a scoped API Token is a bearer credential and needs nothing else.
    # Preferring the token means switching to one later is a matter of clearing
    # CF_API_EMAIL, with no edit here.
    if [[ -n "${CF_API_EMAIL:-}" ]]; then
        CF_AUTH=(-H "X-Auth-Email: $CF_API_EMAIL" -H "X-Auth-Key: $CF_API_KEY")
    else
        CF_AUTH=(-H "Authorization: Bearer $CF_API_KEY")
    fi

    ZONE=$(curl -s "https://api.cloudflare.com/client/v4/zones?name=openimg.io" \
        "${CF_AUTH[@]}" \
        | python3 -c "import sys,json;d=json.load(sys.stdin);print(d['result'][0]['id'] if d.get('success') and d.get('result') else '')")

    if [[ -z "$ZONE" ]]; then
        # Loud, not fatal: the deploy itself succeeded, and a failed purge is a
        # stale-asset problem rather than a broken site. Silent would be worse —
        # that is how the favicons stayed violet for a day.
        printf "  \033[33m!\033[0m 取不到 zone id，缓存未清（凭据无效或权限不足）\n"
    else
        RESP=$(curl -s -X POST "https://api.cloudflare.com/client/v4/zones/$ZONE/purge_cache" \
            "${CF_AUTH[@]}" \
            -H "Content-Type: application/json" --data '{"purge_everything":true}')
        if python3 -c "import sys,json;sys.exit(0 if json.load(sys.stdin).get('success') else 1)" <<<"$RESP"; then
            ok "已清空 openimg.io 缓存"
            sleep 5
        else
            printf "  \033[33m!\033[0m 清缓存失败：%s\n" \
                "$(python3 -c "import sys,json;d=json.load(sys.stdin);print((d.get('errors') or [{}])[0].get('message','未知'))" <<<"$RESP")"
        fi
    fi
else
    printf "  跳过：未设置 CF_API_KEY（见 deploy/.env.local）\n"
    printf "  旧资源可能仍被 CDN 缓存，浏览器请强制刷新\n"
fi

say "7/7  验证"
code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 "https://openimg.io/")
[[ "$code" == "200" ]] && ok "openimg.io  $code" || die "openimg.io  $code"
# CDN 域名根路径设计为 301 回主站(否则 MinIO 会返回整个桶的对象列表),
# 域名健康与否要看它跳得对不对,而不是 200
for host in files.openimgcdn.com cache.openimgcdn.com static.openimg.io; do
    code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 "https://$host/")
    [[ "$code" == "301" ]] && ok "$host  301 →  openimg.io" || die "$host  $code(应为 301)"
done
# 根路径 301 只证明域名活着,不证明 MinIO 那条 rewrite 还对——换 CDN 域名时
# 错的恰恰是后者,而它要等用户点开一张图才会暴露。
#
# 判据是"取一个不存在的对象要拿到 404"。404 说明请求真的走到了 MinIO 并被
# 它拒绝,整条代理链是通的;502/503 说明后面没人接,而 200 说明拿到了桶列表
# ——那是更糟的一种坏。不用真实对象键是为了不依赖任何接口或凭据。
for host in files.openimgcdn.com cache.openimgcdn.com static.openimg.io; do
    code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 \
           "https://$host/deploy-probe-does-not-exist-$$.png")
    [[ "$code" == "404" ]] && ok "$host  对象链路通(404)" \
        || die "$host  探测返回 $code(应为 404,502/503=代理断了,200=桶列表泄露)"
done

code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 "https://www.openimg.io/")
[[ "$code" == "301" ]] && ok "www.openimg.io  301 →  openimg.io" || die "www 跳转异常：$code"

# CDN 域名的裸域与 www 只做跳转,不是站点的第二个入口。单独验:它们用的是另
# 一张证书,而证书没配对时表现是 TLS 握手失败,不是 4xx —— curl 会直接报错。
for host in openimgcdn.com www.openimgcdn.com; do
    code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 "https://$host/")
    [[ "$code" == "301" ]] && ok "$host  301 →  openimg.io" || die "$host  $code(应为 301)"
done

# The check that matters. A 200 here proves nothing on its own: the SPA
# fallback will happily answer 200 with HTML for a .js path, and the browser
# then renders a blank page.
html=$(curl -s --max-time 20 "$SITE/")
for asset in $(grep -oE '/assets/[A-Za-z0-9._-]+' <<<"$html" | sort -u); do
    ct=$(curl -s -o /dev/null -w '%{content_type}' --max-time 20 "$SITE$asset")
    case "$asset:$ct" in
        *.js:text/javascript*|*.js:application/javascript*) ok "$(basename "$asset")  $ct" ;;
        *.css:text/css*)                                    ok "$(basename "$asset")  $ct" ;;
        *) die "$(basename "$asset") 的 Content-Type 是 $ct —— 资源被 SPA 回退吞掉了，页面会是空白" ;;
    esac
done

api=$(curl -s --max-time 20 "$SITE/api/public-stats")
grep -q total_images <<<"$api" && ok "API 正常" || die "API 异常：$api"

say "部署完成"
printf "  %s\n" "$SITE"
printf "  回滚：./deploy/deploy.sh --rollback\n\n"
