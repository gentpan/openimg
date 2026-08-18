package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gentpan/openimg/backend/internal/aigen"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// stubRemote 只需要 Enabled() 是真的:4K 门槛看的是"这个部署接没接 pic.bi"
// 加上"这个人绑没绑",不涉及任何真的扣费。
type stubRemote struct{ on bool }

func (s stubRemote) Enabled() bool                                { return s.on }
func (s stubRemote) Balance(context.Context, string) (int, error) { return 0, nil }
func (s stubRemote) Quote(context.Context, string, string, string) (int, error) {
	return 0, nil
}
func (s stubRemote) Spend(context.Context, string, string, string, string, string) (string, int, error) {
	return "", 0, nil
}
func (s stubRemote) Refund(context.Context, string, string) error { return nil }

// 4K 是逐个用户判的,而且判断只有一处:status 报出去的清单和两个提交入口
// 收进来的校验读的是同一个函数。分成两处的下场是界面上不给选,直接 POST
// 就能过。
func TestAllowedResolutionsFor(t *testing.T) {
	linked := &models.User{ID: uuid.New(), PicbiID: "picbi-1"}
	plain := &models.User{ID: uuid.New()}

	aigen.SetRemote(stubRemote{on: true})
	defer aigen.SetRemote(nil)

	if got := allowedResolutionsFor(plain); len(got) != 2 {
		t.Errorf("未关联用户的档位 = %v, 期望只有 1k/2k", got)
	}
	if resolutionAllowed(plain, "4k") {
		t.Error("未关联用户拿到了 4k")
	}
	if !resolutionAllowed(linked, "4k") {
		t.Error("已关联用户被挡在 4k 之外")
	}
	for _, r := range []string{"1k", "2k"} {
		if !resolutionAllowed(plain, r) || !resolutionAllowed(linked, r) {
			t.Errorf("%s 档位不该被挡", r)
		}
	}

	// 部署没配 partner 凭据时,绑过也扣不了费,4K 必须一并关掉——否则会出现
	// "选得了 4k、扣费时才 503"。
	aigen.SetRemote(stubRemote{on: false})
	if resolutionAllowed(linked, "4k") {
		t.Error("部署未接 pic.bi 时仍放开了 4k")
	}
	aigen.SetRemote(nil)
	if resolutionAllowed(linked, "4k") {
		t.Error("没有远程实现时仍放开了 4k")
	}
}

// 越权的档位要明确报错,不能像原来那样静默回落 1k。
//
// 静默回落有两个后果:未关联用户直接 POST resolution=4k 会被当成 1k 悄悄
// 放行(越权请求以成功的样子返回),而用户以为自己出的是 4K 的图。
func TestPickResolution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	aigen.SetRemote(stubRemote{on: true})
	defer aigen.SetRemote(nil)

	linked := &models.User{ID: uuid.New(), PicbiID: "picbi-1"}
	plain := &models.User{ID: uuid.New()}
	s := &Server{}

	cases := []struct {
		name     string
		user     *models.User
		raw      string
		wantOK   bool
		wantRes  string
		wantCode int
	}{
		// 不传是允许的:客户端可以不带这个字段。
		{"留空按 1k", plain, "", true, "1k", http.StatusOK},
		{"空白按 1k", plain, "   ", true, "1k", http.StatusOK},
		{"正常 2k", plain, "2k", true, "2k", http.StatusOK},
		// 拼错了是客户端的问题 → 400;没资格是权限问题 → 403。两者分开,
		// 客户端才知道该改参数还是该去关联账号。
		{"没听说过的档位", plain, "8k", false, "", http.StatusBadRequest},
		{"未关联用户要 4k", plain, "4k", false, "", http.StatusForbidden},
		{"已关联用户要 4k", linked, "4k", true, "4k", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/ai/generate", nil)

			got, ok := s.pickResolution(c, tc.user, tc.raw)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, 期望 %v", ok, tc.wantOK)
			}
			if ok && got != tc.wantRes {
				t.Errorf("档位 = %q, 期望 %q", got, tc.wantRes)
			}
			if !ok && rec.Code != tc.wantCode {
				t.Errorf("状态码 = %d, 期望 %d", rec.Code, tc.wantCode)
			}
		})
	}
}

// pic.bi 不可达是 503,不是 402。混成一句,用户会按"没钱了"去签到,签了也
// 没用;而在代码里混成一件事,下一步就是有人提议"那不如退回免费额度"——那
// 正是 pic.bi 每抖一次全站就能免费刷 4k 的那个洞。
func TestAIBeginFailedStatusCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{}
	cases := []struct {
		err  error
		want int
	}{
		{aigen.ErrRemoteUnavailable, http.StatusServiceUnavailable},
		{aigen.ErrRemoteDenied, http.StatusForbidden},
		{aigen.ErrDailyLimit, http.StatusTooManyRequests},
		{aigen.ErrNoCredits, http.StatusPaymentRequired},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/ai/generate", nil)
		s.aiBeginFailed(c, tc.err)
		if rec.Code != tc.want {
			t.Errorf("%v → %d, 期望 %d", tc.err, rec.Code, tc.want)
		}
	}
}

// picbi 只能绑、不能用来登录。subjectColumn 认得它,是为了绑定与解绑写得进
// 那一列;而登录那条路由压根没注册,callback 里还有一道明确的拦截。
func TestSubjectColumn(t *testing.T) {
	want := map[string]string{
		"google": "google_sub",
		"github": "github_id",
		"picbi":  "picbi_id",
		"什么":     "",
	}
	for provider, col := range want {
		if got := subjectColumn(provider); got != col {
			t.Errorf("%s → %q, 期望 %q", provider, got, col)
		}
	}
}
