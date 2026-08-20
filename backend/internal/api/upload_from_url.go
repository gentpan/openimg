package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/fetchurl"
	"github.com/gentpan/openimg/backend/internal/i18n"
	"github.com/gin-gonic/gin"
)

// 取一张远程图的时间上限。比上传本身短:这一段是替用户去连一台我们不认识的
// 机器,拖着不放等于占着一个连接和一个 goroutine 等对方高兴。
const fetchURLTimeout = 20 * time.Second

// POST /api/upload/from-url — 按用户给的网址抓一张图存下来。
//
// 网页端不能像 macOS 客户端那样在本地下载:浏览器对第三方图片地址基本都会被
// CORS 挡掉。所以这一步只能由服务端做,而服务端做就意味着**任何登录用户都能
// 让我们的机器去请求任意地址**。防护全在 fetchurl 包里(按 IP 判定、连的就是
// 查过的那个 IP、跳转每一跳都过闸),那里有一张穷举的测试表。
//
// 这里只负责三件事:走和表单上传同一套限额、把字节交给同一条落地管线、以及不
// 把网络细节回显出去——回显等于把这台机器变成一个"这个地址通不通"的探针。
func (s *Server) handleUploadFromURL(c *gin.Context) {
	u := auth.MustUser(c)
	g := s.groupFor(u)
	// 与表单上传同一道闸:邮箱验证、账号状态、每日张数、单文件上限。换一条
	// 路径就绕开限额是这类"第二个入口"最典型的漏法。
	maxSize, ok := s.uploadPreflight(c, u, &g)
	if !ok {
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "upload.missing_field")})
		return
	}
	target, err := fetchurl.Check(strings.TrimSpace(req.URL))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), fetchURLTimeout)
	defer cancel()

	raw, _, err := fetchurl.New(maxSize, fetchURLTimeout).Get(ctx, target)
	if err != nil {
		status := http.StatusBadGateway
		switch {
		case errors.Is(err, fetchurl.ErrTooLarge):
			status = http.StatusRequestEntityTooLarge
		case errors.Is(err, fetchurl.ErrBlocked), errors.Is(err, fetchurl.ErrScheme),
			errors.Is(err, fetchurl.ErrPort), errors.Is(err, fetchurl.ErrHost):
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	sum := sha256.Sum256(raw)
	// 格式不看这个名字,只当展示用——真正的格式由 finishUpload 里的字节嗅探
	// 决定。所以这里不必也不该信任路径里的后缀。
	s.finishUpload(c, u, &g, raw, hex.EncodeToString(sum[:]), displayNameFor(target.Path))
}

// displayNameFor 从网址路径里取一个能看的文件名。
//
// 只取最后一段并且拒绝 "." / ".."：这个名字会一路走到存储层,而路径拼接对
// ".." 是没有免疫力的。
func displayNameFor(urlPath string) string {
	name := path.Base(urlPath)
	if name == "" || name == "/" || name == "." || name == ".." || strings.Contains(name, "..") {
		return "image"
	}
	if len(name) > 100 {
		name = name[:100]
	}
	return name
}
