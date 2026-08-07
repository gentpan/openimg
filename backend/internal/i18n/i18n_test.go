package i18n

import (
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// verbs counts the format placeholders in a message, ignoring "%%".
var verbs = regexp.MustCompile(`%[^%]`)

// The check that makes this catalogue safe to grow.
//
// Adding a Chinese message and forgetting its English twin is the failure this
// whole package exists to prevent, and it is invisible until an English-reading
// user hits that exact code path. Here it is a failing test on the next run.
func TestCataloguesAgree(t *testing.T) {
	for key, zhMsg := range zh {
		enMsg, ok := en[key]
		if !ok {
			t.Errorf("英文目录缺少 %q（中文是 %q）", key, zhMsg)
			continue
		}
		// Mismatched verbs are worse than a missing translation: T passes the
		// same args to both, so one extra %s renders "%!s(MISSING)" into a
		// response, and one too few silently drops a number the reader needed.
		if a, b := len(verbs.FindAllString(zhMsg, -1)), len(verbs.FindAllString(enMsg, -1)); a != b {
			t.Errorf("%q 的占位符数量不一致：中文 %d 个（%q），英文 %d 个（%q）", key, a, zhMsg, b, enMsg)
		}
	}
	for key := range en {
		if _, ok := zh[key]; !ok {
			t.Errorf("英文目录多出 %q，中文没有——大概率是键名写错了", key)
		}
	}
}

func TestNoChineseLeftInEnglish(t *testing.T) {
	cjk := regexp.MustCompile(`[\p{Han}]`)
	for key, msg := range en {
		if cjk.MatchString(msg) {
			t.Errorf("英文目录里 %q 仍含中文：%q", key, msg)
		}
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	cases := map[string]Lang{
		"":                           Default,
		"zh":                         ZH,
		"zh-CN":                      ZH,
		"zh-Hans-CN":                 ZH,
		"en":                         EN,
		"en-GB":                      EN,
		"en-US,en;q=0.9":             EN,
		"zh-CN,zh;q=0.9,en;q=0.8":    ZH,
		"en-US,en;q=0.9,zh-CN;q=0.8": EN,
		// Unknown languages fall back rather than erroring — a Japanese browser
		// gets the default, not a 400.
		"ja,ko;q=0.9": Default,
		"*":           Default,
	}
	for header, want := range cases {
		if got := parse(header); got != want {
			t.Errorf("parse(%q) = %q, want %q", header, got, want)
		}
	}
}

func TestUnknownKeyReturnsKey(t *testing.T) {
	if got := TL(EN, "no.such.key"); got != "no.such.key" {
		t.Errorf("未知键应原样返回，得到 %q", got)
	}
}

func TestMiddlewareAndT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Pick any key that exists, so the test does not depend on one message's
	// wording surviving edits.
	keys := Keys()
	if len(keys) == 0 {
		t.Skip("目录还是空的")
	}
	key := keys[0]

	for header, want := range map[string]Lang{"zh-CN": ZH, "en-US": EN} {
		r := gin.New()
		r.Use(Middleware())
		var got string
		var lang Lang
		r.GET("/x", func(c *gin.Context) {
			lang = Of(c)
			got = T(c, key)
			c.Status(200)
		})
		req := httptest.NewRequest("GET", "/x", nil)
		req.Header.Set("Accept-Language", header)
		r.ServeHTTP(httptest.NewRecorder(), req)

		if lang != want {
			t.Errorf("Accept-Language %q → %q, want %q", header, lang, want)
		}
		if strings.TrimSpace(got) == "" {
			t.Errorf("T 返回了空串（key=%q, lang=%q）", key, want)
		}
	}
}
