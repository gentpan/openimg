// Package aigen 接入 APIMart 的文生图接口。
//
// 只做一件事:把提示词换成一张图的下载地址。配额、扣费、落库、进上传流水线
// 都不在这里——那样这个包就成了第二个 api 包。
package aigen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultBase = "https://api.apimart.ai"
	// DefaultModel 与 docs.apimart.ai 的 gpt-image-2 对齐。
	DefaultModel = "gpt-image-2"
	httpTimeout  = 60 * time.Second
)

// ErrNotConfigured 表示没有配 APIMART_API_KEY。调用方据此把功能整个藏起来,
// 而不是让用户点一下再收到 401。
var ErrNotConfigured = errors.New("aigen: 未配置 APIMART_API_KEY")

type Client struct {
	key  string
	base string
	http *http.Client
}

func New(key, base string) *Client {
	if strings.TrimSpace(base) == "" {
		base = defaultBase
	}
	return &Client{
		key:  strings.TrimSpace(key),
		base: strings.TrimRight(base, "/"),
		http: &http.Client{Timeout: httpTimeout},
	}
}

func (c *Client) Enabled() bool { return c != nil && c.key != "" }

// Req 是一次生成请求的全部输入。
//
// 用结构体而不是继续加位置参数:文生图与图生图在上游是同一个接口、同一段
// 请求体,唯一的差别是多带一个 image_urls。为后者另开一个 SubmitEdit 就要把
// 编码、发请求、取任务号那一串再抄一遍,以后改一处就得记得改两处。
type Req struct {
	Prompt string
	Model  string
	// Size 与 Resolution 留空表示不传给上游。图生图不传 size 时,输出的
	// 尺寸跟随输入图——这正是"修图"想要的默认行为。
	Size       string
	Resolution string
	// ImageURLs 是参考图的公开地址,空则是纯文生图。上游直接去取这些 URL,
	// 所以不必也不该把图下载下来转 base64。
	ImageURLs []string
}

// Submit 递交一次生成,返回上游任务号。上游是异步的:这里只拿到受理回执,
// 图要靠 Poll 取。
func (c *Client) Submit(ctx context.Context, req Req) (string, error) {
	if !c.Enabled() {
		return "", ErrNotConfigured
	}
	payload := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
		"n":      1,
	}
	if req.Size != "" {
		payload["size"] = req.Size
	}
	if req.Resolution != "" {
		payload["resolution"] = req.Resolution
	}
	if len(req.ImageURLs) > 0 {
		payload["image_urls"] = req.ImageURLs
	}
	body, _ := json.Marshal(payload)
	var out struct {
		Code int `json:"code"`
		Data []struct {
			Status string `json:"status"`
			TaskID string `json:"task_id"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1/images/generations", body, &out); err != nil {
		return "", err
	}
	if len(out.Data) == 0 || out.Data[0].TaskID == "" {
		return "", errors.New("aigen: 上游未返回任务号")
	}
	return out.Data[0].TaskID, nil
}

// Result 是一次轮询的结果。Done 为假时调用方应稍后再来。
type Result struct {
	Done  bool
	URL   string
	Error string
}

// Poll 查一次任务状态。上游的四种状态里只有 completed/failed 是终态。
func (c *Client) Poll(ctx context.Context, taskID string) (Result, error) {
	if !c.Enabled() {
		return Result{}, ErrNotConfigured
	}
	var out struct {
		Code int `json:"code"`
		Data struct {
			Status string `json:"status"`
			Error  string `json:"error"`
			Result struct {
				Images []struct {
					// 上游这个字段是数组:一次任务可能产出多张。当前只要
					// 第一张——提交时 n 恒为 1。
					URL []string `json:"url"`
				} `json:"images"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/tasks/"+taskID, nil, &out); err != nil {
		return Result{}, err
	}
	switch out.Data.Status {
	case "completed":
		for _, img := range out.Data.Result.Images {
			if len(img.URL) > 0 && img.URL[0] != "" {
				return Result{Done: true, URL: img.URL[0]}, nil
			}
		}
		return Result{Done: true, Error: "上游报告完成但没有给出图片地址"}, nil
	case "failed":
		msg := out.Data.Error
		if msg == "" {
			msg = "上游生成失败"
		}
		return Result{Done: true, Error: msg}, nil
	default: // submitted / processing
		return Result{}, nil
	}
}

// Download 取回生成的图。上游给的地址会过期(响应里带 expires_at),所以拿到
// 就下,不留到以后。
func (c *Client) Download(ctx context.Context, url string, max int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("aigen: 下载失败 HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, max))
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, out any) error {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 上游的错误码分两类:402/429 是我们该退还额度并让用户稍后再试的,
		// 400 是提示词或参数本身的问题,重试多少次都一样。调用方按
		// StatusError 里的码决定,别在这里替它做主。
		return &StatusError{Code: resp.StatusCode, Body: string(raw)}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// StatusError 保留上游的 HTTP 码,让调用方能区分"该重试"与"白试"。
type StatusError struct {
	Code int
	Body string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("aigen: 上游返回 %d: %s", e.Code, truncate(e.Body, 300))
}

// Retryable 表示这次失败可能是暂时的。402(余额不足)不算——那要人去充值。
func (e *StatusError) Retryable() bool {
	return e.Code == 429 || e.Code >= 500
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
