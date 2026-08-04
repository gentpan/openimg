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

say "1/6  本地校验"
cd "$ROOT/backend"
go build ./... >/dev/null           || die "后端编译失败"
go vet ./internal/... >/dev/null    || die "go vet 失败"
go test ./internal/... >/dev/null   || die "测试失败"
ok "后端 build / vet / test"
cd "$ROOT/frontend"
npx tsc -b --pretty false >/dev/null 2>&1 || die "TypeScript 校验失败"
npm run build >/dev/null 2>&1            || die "前端构建失败"
ok "前端 tsc / build"

say "2/6  凭据扫描"
cd "$ROOT"
if git diff --cached --quiet && git diff --quiet; then :; fi
if grep -rqE "GOCSPX-[A-Za-z0-9_-]{20,}|AKIA[0-9A-Z]{16}" \
        --include="*.go" --include="*.ts" --include="*.tsx" backend/ frontend/src/ 2>/dev/null; then
    die "源码中发现疑似凭据，已中止"
fi
ok "未发现硬编码凭据"

say "3/6  上传"
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

say "4/6  远端编译并切换"
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

say "5/6  清 CDN 缓存"
if [[ -n "${CF_API_KEY:-}" && -n "${CF_API_EMAIL:-}" ]]; then
    ZONE=$(curl -s "https://api.cloudflare.com/client/v4/zones?name=openimg.io" \
        -H "X-Auth-Email: $CF_API_EMAIL" -H "X-Auth-Key: $CF_API_KEY" \
        | python3 -c "import sys,json;print(json.load(sys.stdin)['result'][0]['id'])")
    curl -s -X POST "https://api.cloudflare.com/client/v4/zones/$ZONE/purge_cache" \
        -H "X-Auth-Email: $CF_API_EMAIL" -H "X-Auth-Key: $CF_API_KEY" \
        -H "Content-Type: application/json" --data '{"purge_everything":true}' >/dev/null
    ok "已清空 openimg.io 缓存"
    sleep 5
else
    printf "  跳过：未设置 CF_API_KEY / CF_API_EMAIL\n"
    printf "  旧资源可能仍被 CDN 缓存，浏览器请强制刷新\n"
fi

say "6/6  验证"
for host in openimg.io cdn.imgla.com cache.imgla.com; do
    code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 "https://$host/")
    [[ "$code" == "200" ]] && ok "$host  $code" || die "$host  $code"
done
code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 "https://www.openimg.io/")
[[ "$code" == "301" ]] && ok "www.openimg.io  301 →  openimg.io" || die "www 跳转异常：$code"

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
