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
	r.GET(macUpdatePath, func(c *gin.Context) {
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
	})
}
