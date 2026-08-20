package api

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// macOS 客户端的更新清单。
//
// 显式路由,而不是把文件丢进 FRONTEND_DIR —— 理由与 apple_assoc.go 完全相同:
// r.NoRoute 会用 index.html 回答任何未匹配路径,状态码 200。客户端如果只检查
// 状态码,就会把一份 HTML 当成合法清单,表现是「静默地永远没有更新」。那是个
// 不报错的失败模式,没人会在发布当天发现。
//
// 清单本身带 Ed25519 签名,所以这条路由不做鉴权、不做任何加工——它只是把一个
// 文件原样吐出来。篡改由客户端的验签挡,不由这里挡。
//
// 顺带一提:这条接口是 AllowAllOrigins 的(见 router.go 的 CORS 配置),任何网页
// 都能读它。内容本来就是要公开的,但别让下一个人以为它有边界。
const macUpdatePath = "/api/app/mac/update.json"

// registerMacUpdate 挂上清单;没配置或文件不在时,在同一个路径上挂一个明确的
// 404 —— 而不是让它落进 NoRoute 拿到 200 + HTML。
func (s *Server) registerMacUpdate(r *gin.Engine) {
	// GET 和 HEAD 都要挂。Gin 不会因为注册了 GET 就自动接 HEAD,而没接的那个
	// 动词会落进 NoRoute —— 于是 `curl -I` 拿到 200 + text/html,看着像"活着",
	// 其实是兜底在答。任何用 HEAD 探活的监控都会被这个骗过去。
	//
	// 发现它纯属侥幸:验证线上部署时顺手用了 curl -sI,结果头是 text/html 而正文
	// 是正确的清单——这个矛盾才逼出了真正的原因。
	h := func(c *gin.Context) {
		if s.MacUpdateManifest == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not configured"})
			return
		}
		b, err := os.ReadFile(s.MacUpdateManifest)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		// 短缓存。清单有签名,中间缓存改不了它,但**能把它冻住**——而撤回一个
		// 坏版本恰恰要靠推一份新清单。所以 TTL 不能长。
		c.Header("Cache-Control", "public, max-age=300")
		c.Data(http.StatusOK, "application/json; charset=utf-8", b)
	}
	r.GET(macUpdatePath, h)
	r.HEAD(macUpdatePath, h)
}
