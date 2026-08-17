package migrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/storage"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// getJSON 是三个 HTTP 来源共用的取数口。
//
// 所有用户填进来的站点地址都先过 ValidateUserEndpoint:否则"自己输入 API
// 地址"这个功能就成了一台替攻击者探测内网的机器(云元数据 169.254.169.254
// 是最经典的目标)。这与 BYOS 存储端点用的是同一道闸。
func getJSON(ctx context.Context, rawURL, authHeader, authValue string, out any) error {
	if err := storage.ValidateUserEndpoint(rawURL); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	if authHeader != "" {
		req.Header.Set(authHeader, authValue)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("migrate: 来源返回 %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return json.Unmarshal(body, out)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func nameFromURL(raw, fallback string) string {
	if fallback != "" {
		return fallback
	}
	if u, err := url.Parse(raw); err == nil {
		if b := path.Base(u.Path); b != "" && b != "/" && b != "." {
			return b
		}
	}
	return "imported"
}

// ---------------------------------------------------------------- SM.MS

// smmsSource 走 SM.MS 的上传历史接口。
//
// 它的分页是页码,一页固定 100 条;没有总数,靠"这一页空了"判断到头。
type smmsSource struct {
	token string
	base  string
}

func newSMMS(cfg Config) (Source, error) {
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("migrate: SM.MS 需要 API Token")
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	if base == "" {
		base = "https://sm.ms"
	}
	return &smmsSource{token: strings.TrimSpace(cfg.Token), base: base}, nil
}

func (s *smmsSource) Kind() string { return KindSMMS }

func (s *smmsSource) List(ctx context.Context, cursor string, _ int) (Page, error) {
	page := 1
	if cursor != "" {
		if n, err := strconv.Atoi(cursor); err == nil && n > 0 {
			page = n
		}
	}
	var out struct {
		Success bool `json:"success"`
		Message string `json:"message"`
		Data    []struct {
			Filename string `json:"filename"`
			URL      string `json:"url"`
			Hash     string `json:"hash"`
		} `json:"data"`
	}
	u := fmt.Sprintf("%s/api/v2/upload_history?page=%d", s.base, page)
	if err := getJSON(ctx, u, "Authorization", s.token, &out); err != nil {
		return Page{}, err
	}
	if !out.Success && len(out.Data) == 0 && out.Message != "" {
		return Page{}, errors.New("migrate: SM.MS: " + out.Message)
	}
	items := make([]Item, 0, len(out.Data))
	for _, d := range out.Data {
		if d.URL == "" {
			continue
		}
		items = append(items, Item{URL: d.URL, Name: d.Filename, ExternalID: d.Hash})
	}
	next := ""
	if len(items) > 0 {
		next = strconv.Itoa(page + 1)
	}
	return Page{Items: items, Cursor: next}, nil
}

// ---------------------------------------------------------------- 兰空图床

// lskySource 走兰空图床 (Lsky Pro) 的图片列表接口。自建站点很多,所以站点
// 地址必须由用户给,且同样过 SSRF 闸。
type lskySource struct {
	token string
	base  string
}

func newLsky(cfg Config) (Source, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	if base == "" {
		return nil, errors.New("migrate: 兰空图床需要站点地址")
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("migrate: 兰空图床需要 Token")
	}
	return &lskySource{token: strings.TrimSpace(cfg.Token), base: base}, nil
}

func (s *lskySource) Kind() string { return KindLsky }

func (s *lskySource) List(ctx context.Context, cursor string, _ int) (Page, error) {
	page := 1
	if cursor != "" {
		if n, err := strconv.Atoi(cursor); err == nil && n > 0 {
			page = n
		}
	}
	var out struct {
		Status bool `json:"status"`
		Message string `json:"message"`
		Data   struct {
			CurrentPage int `json:"current_page"`
			LastPage    int `json:"last_page"`
			Data        []struct {
				Key      string `json:"key"`
				Name     string `json:"name"`
				Origin   string `json:"origin_name"`
				Links    struct {
					URL string `json:"url"`
				} `json:"links"`
			} `json:"data"`
		} `json:"data"`
	}
	token := s.token
	if !strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = "Bearer " + token
	}
	u := fmt.Sprintf("%s/api/v1/images?page=%d", s.base, page)
	if err := getJSON(ctx, u, "Authorization", token, &out); err != nil {
		return Page{}, err
	}
	if !out.Status && len(out.Data.Data) == 0 && out.Message != "" {
		return Page{}, errors.New("migrate: 兰空图床: " + out.Message)
	}
	items := make([]Item, 0, len(out.Data.Data))
	for _, d := range out.Data.Data {
		link := d.Links.URL
		if link == "" {
			continue
		}
		name := d.Origin
		if name == "" {
			name = d.Name
		}
		items = append(items, Item{URL: link, Name: name, ExternalID: d.Key})
	}
	next := ""
	if out.Data.LastPage > page && len(items) > 0 {
		next = strconv.Itoa(page + 1)
	}
	return Page{Items: items, Cursor: next}, nil
}

// ---------------------------------------------------------------- S3 / R2

// s3Source 列举一个桶里的对象。
//
// 与 BYOS 的区别:BYOS 是"以后传到这里",迁移是"把已经在这里的搬进来"。
// 两者都用同一套 S3 客户端与同一道 endpoint 闸。
type s3Source struct {
	lister storage.Lister
	prefix string
}

func newS3(cfg Config) (Source, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("migrate: 需要桶名")
	}
	l, err := storage.NewLister(storage.ListerConfig{
		Endpoint:  cfg.Endpoint,
		Region:    cfg.Region,
		Bucket:    cfg.Bucket,
		AccessKey: cfg.AccessKey,
		SecretKey: cfg.SecretKey,
	})
	if err != nil {
		return nil, err
	}
	return &s3Source{lister: l, prefix: cfg.Prefix}, nil
}

func (s *s3Source) Kind() string { return KindS3 }

func (s *s3Source) List(ctx context.Context, cursor string, limit int) (Page, error) {
	if limit <= 0 {
		limit = 200
	}
	objs, next, err := s.lister.List(ctx, s.prefix, cursor, limit)
	if err != nil {
		return Page{}, err
	}
	items := make([]Item, 0, len(objs))
	for _, o := range objs {
		// 目录占位对象与非图片扩展名直接跳过——省得为每个 .txt 发一次请求
		// 才发现它不是图。真正的格式判定仍在入库前按字节嗅探。
		if strings.HasSuffix(o.Key, "/") || !looksLikeImage(o.Key) {
			continue
		}
		items = append(items, Item{
			URL:        o.URL,
			Name:       path.Base(o.Key),
			ExternalID: o.Key,
		})
	}
	return Page{Items: items, Cursor: next}, nil
}

func looksLikeImage(key string) bool {
	ext := strings.ToLower(path.Ext(key))
	switch ext {
	case ".jpg", ".jpeg", ".jpe", ".png", ".gif", ".webp", ".avif",
		".heic", ".heif", ".bmp", ".tif", ".tiff":
		return true
	}
	return false
}

// ---------------------------------------------------------------- 地址清单

// urlListSource 是兜底的那一种:用户从任何地方导出一份地址清单粘进来。
//
// 有了它,"支持某某图床"就不再是必须逐个适配的事——任何能导出链接的服务
// 都能搬过来。
type urlListSource struct{ urls []string }

func newURLList(cfg Config) (Source, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(cfg.URLs))
	for _, raw := range cfg.URLs {
		u := strings.TrimSpace(raw)
		if u == "" || seen[u] {
			continue
		}
		if err := storage.ValidateUserEndpoint(u); err != nil {
			return nil, fmt.Errorf("migrate: %s: %w", truncate(u, 60), err)
		}
		seen[u] = true
		out = append(out, u)
	}
	if len(out) == 0 {
		return nil, errors.New("migrate: 清单里没有可用的地址")
	}
	return &urlListSource{urls: out}, nil
}

func (s *urlListSource) Kind() string { return KindURLList }

func (s *urlListSource) List(_ context.Context, cursor string, limit int) (Page, error) {
	if limit <= 0 {
		limit = 200
	}
	start := 0
	if cursor != "" {
		if n, err := strconv.Atoi(cursor); err == nil && n > 0 {
			start = n
		}
	}
	if start >= len(s.urls) {
		return Page{}, nil
	}
	end := min(start+limit, len(s.urls))
	items := make([]Item, 0, end-start)
	for _, u := range s.urls[start:end] {
		items = append(items, Item{URL: u, Name: nameFromURL(u, "")})
	}
	next := ""
	if end < len(s.urls) {
		next = strconv.Itoa(end)
	}
	return Page{Items: items, Cursor: next}, nil
}
