package picbi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// 签名必须覆盖请求体。只签时间戳和 nonce 的话,那串签名就只是一张与内容无关
// 的通行证——中间人可以原样保留三个头,把 user_id 换成别人的。
func TestSignCoversBody(t *testing.T) {
	a := Sign("s3cret", "1700000000", "abc", []byte(`{"user_id":"u1"}`))
	b := Sign("s3cret", "1700000000", "abc", []byte(`{"user_id":"u2"}`))
	if a == b {
		t.Fatal("改了请求体签名却没变")
	}
	// 换密钥、换 nonce、换时间戳同样都要变签名。
	if Sign("other", "1700000000", "abc", nil) == Sign("s3cret", "1700000000", "abc", nil) {
		t.Error("换了密钥签名没变")
	}
	if Sign("s3cret", "1700000000", "abc", nil) == Sign("s3cret", "1700000000", "xyz", nil) {
		t.Error("换了 nonce 签名没变")
	}
	// 分隔符不能省:没有它,("1","23") 和 ("12","3") 会签出同一个值。
	if Sign("k", "1", "23", nil) == Sign("k", "12", "3", nil) {
		t.Error("时间戳与 nonce 之间缺少分隔,存在歧义拼接")
	}
}

func TestSpendSignsAndCarriesNoAmount(t *testing.T) {
	const secret = "shared-secret"
	var gotBody []byte
	var gotHeader http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotHeader = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"op_id":"op-1","credits":4,"balance_after":96}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "openimg", secret)
	opID, credits, err := c.Spend(context.Background(), "picbi-1", "gpt-image-2", "4k", "openimg:gen:x", "AI 生成：猫")
	if err != nil {
		t.Fatalf("扣费失败: %v", err)
	}
	if opID != "op-1" || credits != 4 {
		t.Fatalf("返回 = (%q, %d), 期望 (op-1, 4)", opID, credits)
	}

	// 服务端算出来的签名必须和头里那个对得上,而且签的正是收到的这串字节。
	ts := gotHeader.Get("X-Partner-Timestamp")
	nonce := gotHeader.Get("X-Partner-Nonce")
	if want := Sign(secret, ts, nonce, gotBody); gotHeader.Get("X-Partner-Signature") != want {
		t.Error("签名与「时间戳+nonce+原始请求体」对不上")
	}
	if gotHeader.Get("X-Partner-Id") != "openimg" {
		t.Errorf("X-Partner-Id = %q", gotHeader.Get("X-Partner-Id"))
	}
	// 时间戳得是秒级 Unix,对端要拿它比五分钟的窗口。
	if n, err := strconv.ParseInt(ts, 10, 64); err != nil || time.Since(time.Unix(n, 0)) > time.Minute {
		t.Errorf("时间戳不可用: %q", ts)
	}
	if len(nonce) < 16 {
		t.Errorf("nonce 太短,防重放靠它去重: %q", nonce)
	}

	// 请求体里绝不能出现金额。该扣几点是 pic.bi 按 (model, resolution) 自己
	// 算的;留一个能传金额的字段,就等于把定价权交给了调用方。
	var payload map[string]any
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("请求体不是 JSON: %v", err)
	}
	for _, banned := range []string{"amount", "credits", "cost", "price"} {
		if _, ok := payload[banned]; ok {
			t.Errorf("请求体里出现了金额字段 %q", banned)
		}
	}
	for _, want := range []string{"user_id", "model", "resolution", "idempotency_key"} {
		if _, ok := payload[want]; !ok {
			t.Errorf("请求体缺少 %q", want)
		}
	}
}

// 退款只带流水号,不带金额:金额由 pic.bi 从原扣费流水里读,否则调用方可以
// 退出比扣掉的更多的钱。
func TestRefundSendsOnlyOpID(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"credits":4,"balance_after":100}`))
	}))
	defer srv.Close()

	if err := New(srv.URL, "openimg", "k").Refund(context.Background(), "op-1", "openimg:refund:x"); err != nil {
		t.Fatalf("退款失败: %v", err)
	}
	var payload map[string]any
	_ = json.Unmarshal(gotBody, &payload)
	if payload["op_id"] != "op-1" {
		t.Errorf("op_id = %v", payload["op_id"])
	}
	if len(payload) != 2 {
		t.Errorf("退款请求体多带了字段: %v", payload)
	}
}

// 状态码的分界线不是「成功/失败」,而是「结果确定/结果未知」。
func TestErrorKinds(t *testing.T) {
	cases := map[int]string{
		http.StatusPaymentRequired:     KindNoCredits,
		http.StatusForbidden:           KindForbidden,
		http.StatusUnauthorized:        KindForbidden,
		http.StatusBadRequest:          KindInvalid,
		http.StatusConflict:            KindConflict,
		http.StatusInternalServerError: KindUnavailable,
		http.StatusBadGateway:          KindUnavailable,
		http.StatusTooManyRequests:     KindUnavailable,
		http.StatusServiceUnavailable:  KindUnavailable,
		http.StatusGatewayTimeout:      KindUnavailable,
	}
	for code, want := range cases {
		if got := kindForStatus(code); got != want {
			t.Errorf("HTTP %d → %q, 期望 %q", code, got, want)
		}
	}
}

func TestSpendMapsUpstreamStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"error":"余额不足"}`))
	}))
	defer srv.Close()

	_, _, err := New(srv.URL, "openimg", "k").Spend(context.Background(), "u", "m", "1k", "key", "r")
	pe, ok := err.(*Error)
	if !ok {
		t.Fatalf("错误类型 = %T, 期望 *picbi.Error", err)
	}
	if pe.RemoteKind() != KindNoCredits {
		t.Errorf("种类 = %q, 期望 %q", pe.RemoteKind(), KindNoCredits)
	}
	if pe.Message != "余额不足" {
		t.Errorf("消息 = %q, 期望取出 error 字段", pe.Message)
	}
}

// spend 回了 200 但没给流水号 = 这笔钱以后退不回来。当成失败上报,好过留一笔
// 退不了的扣费。
func TestSpendWithoutOpIDIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"credits":1,"balance_after":9}`))
	}))
	defer srv.Close()
	if _, _, err := New(srv.URL, "openimg", "k").Spend(context.Background(), "u", "m", "1k", "key", "r"); err == nil {
		t.Fatal("没有 op_id 却当成功返回")
	}
}

// 连不上的时候必须报 unavailable,不能是别的:调用方据此回 503,而不是
// 放行一次免费生成。
func TestUnreachableIsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // 关掉再打,制造一次连接失败

	_, err := New(url, "openimg", "k").Balance(context.Background(), "u")
	pe, ok := err.(*Error)
	if !ok {
		t.Fatalf("错误类型 = %T", err)
	}
	if pe.RemoteKind() != KindUnavailable {
		t.Errorf("种类 = %q, 期望 %q", pe.RemoteKind(), KindUnavailable)
	}
}

// 三个凭据缺一个就当没配:半配置状态下每次调用都会被对端拒绝,而那看起来
// 像是「pic.bi 坏了」。
func TestEnabledNeedsAllThree(t *testing.T) {
	if New("https://pic.bi", "openimg", "k").Enabled() != true {
		t.Error("配齐了却报未启用")
	}
	for _, c := range []*Client{
		New("", "openimg", "k"),
		New("https://pic.bi", "", "k"),
		New("https://pic.bi", "openimg", ""),
		nil,
	} {
		if c.Enabled() {
			t.Error("缺凭据却报已启用")
		}
	}
}

func TestBalanceUsesQueryAndDisabledClientFailsClosed(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("user_id")
		_, _ = w.Write([]byte(`{"balance":12}`))
	}))
	defer srv.Close()
	n, err := New(srv.URL, "openimg", "k").Balance(context.Background(), "picbi-9")
	if err != nil || n != 12 {
		t.Fatalf("余额 = (%d, %v)", n, err)
	}
	if gotQuery != "picbi-9" {
		t.Errorf("user_id = %q", gotQuery)
	}

	// 没配的客户端不能悄悄返回零值:零余额和"问不到"是两回事。
	if _, err := New("", "", "").Balance(context.Background(), "u"); err == nil {
		t.Error("未配置的客户端返回了成功")
	}
}
