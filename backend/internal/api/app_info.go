package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 下载页要的那些数字。
//
// 两个来源合起来:版本、体积、下载地址取自本地那份更新清单——它是发布流程自己
// 写下的,最可信,而且 GitHub 挂了它照样在;下载次数和更新日志只能问 GitHub。
// 主数据不依赖外网,是因为下载页不该因为 api.github.com 抽风就打不开。
const (
	ghOwnerRepo = "gentpan/openimg-app"

	// GitHub 未登录调用是每小时 60 次(按 IP)。缓存一小时,顺带把下载页从
	// "每次访问都打一次外网"变成"每小时一次"。
	ghCacheTTL = time.Hour
)

type macRelease struct {
	Version    string   `json:"version"`
	Date       string   `json:"date"`
	Highlights []string `json:"highlights"`
	URL        string   `json:"url"`
}

type macInfo struct {
	Version     string `json:"version"`
	Build       int64  `json:"build"`
	Size        int64  `json:"size"`
	ZipURL      string `json:"zip_url"`
	DMGURL      string `json:"dmg_url,omitempty"`
	MinSystem   string `json:"minimum_system"`
	PublishedAt string `json:"published_at"`

	// Downloads 是所有版本累计。分版本看意义不大——大多数人只会下最新的那个,
	// 老版本的数字只反映它当时在架多久。
	Downloads int          `json:"downloads"`
	History   []macRelease `json:"history"`
}

// 清单 payload 里我们关心的那几项。它是发布脚本自己生成的,所以这里只解不验签
// ——验签是客户端的事,那一步防的是"传输中被换掉",而这里读的是本机文件。
type manifestPayload struct {
	Latest struct {
		Version   string `json:"version"`
		Build     int64  `json:"build"`
		Size      int64  `json:"size"`
		URL       string `json:"url"`
		MinSystem string `json:"minimumSystemVersion"`
	} `json:"latest"`
	IssuedAt string `json:"issuedAt"`
}

type ghRelease struct {
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
	Body        string `json:"body"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	Assets      []struct {
		Name          string `json:"name"`
		DownloadCount int    `json:"download_count"`
		URL           string `json:"browser_download_url"`
	} `json:"assets"`
}

var ghCache struct {
	sync.Mutex
	at   time.Time
	data []ghRelease
}

// fetchReleases 取 GitHub 上的发布列表,带缓存。
//
// 取不到就返回上一次的结果(哪怕已经过期):一个稍旧的下载次数,比页面上突然
// 空掉一块要好得多。
func fetchReleases(ctx context.Context) []ghRelease {
	ghCache.Lock()
	fresh := time.Since(ghCache.at) < ghCacheTTL && ghCache.data != nil
	cached := ghCache.data
	ghCache.Unlock()
	if fresh {
		return cached
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/repos/"+ghOwnerRepo+"/releases?per_page=10", nil)
	if err != nil {
		return cached
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return cached
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return cached
	}
	var out []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return cached
	}

	ghCache.Lock()
	ghCache.at, ghCache.data = time.Now(), out
	ghCache.Unlock()
	return out
}

// 发布说明里每条要点都写成 `- **一句话**。后面是展开`。下载页只要那句加粗的
// ——展开部分是写给要读 CHANGELOG 的人的,铺在落地页上只会把它压垮。
var boldLead = regexp.MustCompile(`^\s*[-*]\s+\*\*(.+?)\*\*`)

func highlightsOf(body string, max int) []string {
	// 初始化成空切片而不是 nil:Go 把 nil slice 序列化成 JSON 的 null,而不是
	// []。早期几个版本的发布说明没有 `- **要点**` 这种写法,于是它们的
	// highlights 是 null,前端一读 .length 整页白屏。
	out := []string{}
	for _, line := range strings.Split(body, "\n") {
		m := boldLead.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		s := strings.TrimRight(strings.TrimSpace(m[1]), "。.:：")
		if s == "" {
			continue
		}
		out = append(out, s)
		if len(out) >= max {
			break
		}
	}
	return out
}

// GET /api/app/mac/info — 下载页要的一切。
func (s *Server) handleMacInfo(c *gin.Context) {
	info := macInfo{History: []macRelease{}}

	// 主数据:本地清单。
	if s.MacUpdateManifest != "" {
		if b, err := readManifestPayload(s.MacUpdateManifest); err == nil {
			info.Version = b.Latest.Version
			info.Build = b.Latest.Build
			info.Size = b.Latest.Size
			info.ZipURL = b.Latest.URL
			info.MinSystem = b.Latest.MinSystem
			info.PublishedAt = b.IssuedAt
		}
	}

	// 补充数据:GitHub。
	rels := fetchReleases(c.Request.Context())
	for _, r := range rels {
		if r.Draft || r.Prerelease {
			continue
		}
		for _, a := range r.Assets {
			info.Downloads += a.DownloadCount
			// 推荐 dmg:zip 解压后就地双击会触发 App Translocation,那种状态下
			// 应用内自我更新会永久失效。dmg 逼着人先拖进「应用程序」。
			if strings.EqualFold(strings.TrimPrefix(r.TagName, "v"), info.Version) &&
				strings.HasSuffix(strings.ToLower(a.Name), ".dmg") {
				info.DMGURL = a.URL
			}
		}
		date := r.PublishedAt
		if len(date) >= 10 {
			date = date[:10]
		}
		info.History = append(info.History, macRelease{
			Version:    strings.TrimPrefix(r.TagName, "v"),
			Date:       date,
			Highlights: highlightsOf(r.Body, 4),
			URL:        r.HTMLURL,
		})
	}

	// 清单读不到时(还没发过版)退回 GitHub 上最新那个,总比整页空着强。
	if info.Version == "" && len(info.History) > 0 {
		info.Version = info.History[0].Version
		info.PublishedAt = info.History[0].Date
	}

	c.Header("Cache-Control", "public, max-age=600")
	c.JSON(http.StatusOK, info)
}

func readManifestPayload(path string) (manifestPayload, error) {
	var out manifestPayload
	b, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	var envelope struct {
		Payload string `json:"payload"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		return out, err
	}
	raw, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(raw, &out)
	return out, err
}
