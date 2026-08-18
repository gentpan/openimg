// Package picbi 是 pic.bi 的 partner 客户端:查余额、报价、扣费、退款。
//
// 分工:pic.bi 是账本与身份的权威方(它那边有真实收款和流水),openimg 只是
// 一个客户端。所以这个包里没有任何"这次该扣几点"的判断——金额一律由 pic.bi
// 按 (model, resolution) 自己算,请求体里根本没有 amount 这个字段。多一个
// 可以传金额的口子,就等于把定价权交给了调用方。
//
// 认证是服务端到服务端的 HMAC-SHA256,签名内容是
//
//	timestamp + "\n" + nonce + "\n" + 原始请求体
//
// 覆盖请求体是关键:少了它,签名就只是一张与内容无关的通行证,中间人可以在
// 保持头不变的前提下改掉 user_id。nonce 由对端落库去重,时间戳给出五分钟的
// 窗口,两者一起挡重放。
package picbi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const httpTimeout = 20 * time.Second

// 出错的种类。调用方(aigen)只认这几个字符串,不认 HTTP 状态码——状态码是
// 传输层的细节,而"钱不够"和"服务挂了"必须是两种截然不同的处理。
const (
	KindUnavailable = "unavailable" // 网络不通、超时、5xx、限流:结果未知
	KindNoCredits   = "no_credits"  // 余额不足
	KindForbidden   = "forbidden"   // 没有有效授权 / 用户被停用
	KindInvalid     = "invalid"     // 参数不对(未知 model、不支持的档位)
	KindConflict    = "conflict"    // 退款超过原扣费额之类
)

// Error 带着种类往上走。用 RemoteKind() 而不是导出类型断言,是为了让 aigen
// 不必 import 这个包——两边只靠一个方法名对接,依赖是单向的。
type Error struct {
	Kind    string
	Status  int
	Message string
}

func (e *Error) Error() string {
	if e.Status == 0 {
		return fmt.Sprintf("picbi: %s: %s", e.Kind, e.Message)
	}
	return fmt.Sprintf("picbi: %s (HTTP %d): %s", e.Kind, e.Status, e.Message)
}

// RemoteKind 是 aigen.remoteKinder 那一侧要的方法。
func (e *Error) RemoteKind() string { return e.Kind }

type Client struct {
	base      string
	partnerID string
	secret    string
	http      *http.Client
}

func New(base, partnerID, secret string) *Client {
	return &Client{
		base:      strings.TrimRight(strings.TrimSpace(base), "/"),
		partnerID: strings.TrimSpace(partnerID),
		secret:    strings.TrimSpace(secret),
		http:      &http.Client{Timeout: httpTimeout},
	}
}

// Enabled 三个都配齐才算开。少一个就当没配:半配置状态下每次调用都会 401,
// 而调用方会把 401 当成"pic.bi 拒绝",用户看到的是一句莫名其妙的报错。
func (c *Client) Enabled() bool {
	return c != nil && c.base != "" && c.partnerID != "" && c.secret != ""
}

// Balance 查某个 pic.bi 用户还有多少积分。
func (c *Client) Balance(ctx context.Context, picbiUserID string) (int, error) {
	var out struct {
		Balance int `json:"balance"`
	}
	q := url.Values{"user_id": {picbiUserID}}
	if err := c.do(ctx, http.MethodGet, "/api/partner/credits?"+q.Encode(), nil, &out); err != nil {
		return 0, err
	}
	return out.Balance, nil
}

// Quote 问一次"这么生成要几点"。只读,不动账。
func (c *Client) Quote(ctx context.Context, picbiUserID, model, resolution string) (int, error) {
	body := map[string]string{"user_id": picbiUserID, "model": model, "resolution": resolution}
	var out struct {
		Credits int `json:"credits"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/partner/credits/quote", body, &out); err != nil {
		return 0, err
	}
	return out.Credits, nil
}

// Spend 扣一次费,返回流水号、实际扣的点数和扣完之后的余额。
//
// 请求里没有金额:pic.bi 按 (model, resolution) 自己算,算不出来就 400。
// idempotencyKey 必须由调用方按业务对象确定性地生成(这里是生成记录的 ID),
// 重试打到同一个键会原样返回上一次的结果,不会扣第二次。
func (c *Client) Spend(ctx context.Context, picbiUserID, model, resolution, idempotencyKey, reason string) (opID string, credits int, err error) {
	body := map[string]string{
		"user_id":         picbiUserID,
		"model":           model,
		"resolution":      resolution,
		"idempotency_key": idempotencyKey,
		"reason":          reason,
	}
	var out struct {
		OpID         string `json:"op_id"`
		Credits      int    `json:"credits"`
		BalanceAfter int    `json:"balance_after"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/partner/credits/spend", body, &out); err != nil {
		return "", 0, err
	}
	if out.OpID == "" {
		// 没有流水号就退不了款。当成失败上报,好过留一笔退不回来的扣费。
		return "", 0, &Error{Kind: KindUnavailable, Message: "spend 未返回 op_id"}
	}
	return out.OpID, out.Credits, nil
}

// Refund 按流水号原路退。
//
// 只传 op_id,不传金额:金额由 pic.bi 从原扣费流水里读。允许调用方指定金额
// 就等于允许它退出比扣掉的更多的钱。
func (c *Client) Refund(ctx context.Context, opID, idempotencyKey string) error {
	body := map[string]string{"op_id": opID, "idempotency_key": idempotencyKey}
	var out struct {
		Credits      int `json:"credits"`
		BalanceAfter int `json:"balance_after"`
	}
	return c.do(ctx, http.MethodPost, "/api/partner/credits/refund", body, &out)
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	if !c.Enabled() {
		return &Error{Kind: KindUnavailable, Message: "未配置 pic.bi partner 凭据"}
	}

	// 先编码成字节再签:签的必须是真正发出去的那一串,不能是"再序列化一次
	// 应该也一样"的另一份。
	var raw []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return &Error{Kind: KindInvalid, Message: err.Error()}
		}
		raw = b
	}

	req, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(raw))
	if err != nil {
		return &Error{Kind: KindInvalid, Message: err.Error()}
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := randHex(16)
	req.Header.Set("X-Partner-Id", c.partnerID)
	req.Header.Set("X-Partner-Timestamp", ts)
	req.Header.Set("X-Partner-Nonce", nonce)
	req.Header.Set("X-Partner-Signature", Sign(c.secret, ts, nonce, raw))
	req.Header.Set("Accept", "application/json")
	if raw != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// 连不上/超时:这一笔的结果是未知的,绝不能当成"没扣成"。调用方
		// 据此返回 503,而不是放行一次免费生成。
		return &Error{Kind: KindUnavailable, Message: err.Error()}
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 300 {
		return &Error{
			Kind:    kindForStatus(resp.StatusCode),
			Status:  resp.StatusCode,
			Message: errMessage(payload),
		}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return &Error{Kind: KindUnavailable, Status: resp.StatusCode, Message: "响应不是合法 JSON: " + err.Error()}
	}
	return nil
}

// Sign 是签名算法本身,导出是为了让测试(和将来的对端排错)能算同一个值。
func Sign(secret, timestamp, nonce string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("\n"))
	mac.Write([]byte(nonce))
	mac.Write([]byte("\n"))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// kindForStatus 把 HTTP 状态码翻译成处理方式。
//
// 分界线不是"成功/失败",而是"结果确定/结果未知":4xx 是对端明确读懂了请求
// 并拒绝,可以按业务处理;429 与 5xx 是对端此刻答不了,必须按不可达处理,
// 因为这一笔到底扣没扣掉谁也不知道。
func kindForStatus(code int) string {
	switch code {
	case http.StatusPaymentRequired:
		return KindNoCredits
	case http.StatusUnauthorized, http.StatusForbidden:
		return KindForbidden
	case http.StatusBadRequest, http.StatusNotFound, http.StatusUnprocessableEntity:
		return KindInvalid
	case http.StatusConflict:
		return KindConflict
	}
	return KindUnavailable
}

// errMessage 尽量从 {"error": "..."} 里取一句人话,取不到就把原文截短带上。
func errMessage(payload []byte) string {
	var e struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(payload, &e) == nil {
		if e.Error != "" {
			return e.Error
		}
		if e.Message != "" {
			return e.Message
		}
	}
	s := strings.TrimSpace(string(payload))
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
