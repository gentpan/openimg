package aigen

import (
	"context"
	"errors"
	"testing"

	"time"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func timeNow() time.Time { return time.Now() }

// fakeRemote 记下被调用了什么。测试要断言的多半不是返回值,而是"这一步到底
// 有没有发生"——比如本地账本的记录绝不该走到远程退款上去。
type fakeRemote struct {
	enabled bool

	spendCalls  int
	refundCalls int

	lastRefundOp  string
	lastRefundKey string
	lastSpendUser string

	spendOpID   string
	spendCredit int
	spendErr    error
	refundErr   error
}

func (f *fakeRemote) Enabled() bool { return f.enabled }

func (f *fakeRemote) Balance(ctx context.Context, uid string) (int, error) { return 0, nil }

func (f *fakeRemote) Quote(ctx context.Context, uid, model, res string) (int, error) {
	return 0, nil
}

func (f *fakeRemote) Spend(ctx context.Context, uid, model, res, key, reason string) (string, int, error) {
	f.spendCalls++
	f.lastSpendUser = uid
	return f.spendOpID, f.spendCredit, f.spendErr
}

func (f *fakeRemote) Refund(ctx context.Context, opID, key string) error {
	f.refundCalls++
	f.lastRefundOp, f.lastRefundKey = opID, key
	return f.refundErr
}

// kindErr 冒充 picbi.Error:aigen 只认 RemoteKind() 这一个方法,所以测试不必
// 把真正的客户端拖进来。
type kindErr struct{ kind string }

func (e *kindErr) Error() string      { return "picbi: " + e.kind }
func (e *kindErr) RemoteKind() string { return e.kind }

// 分流规则:本地够用就不碰 pic.bi;本地用完了,关联过的人才有下一条路。
func TestChooseLedger(t *testing.T) {
	linked := &models.User{ID: uuid.New(), PicbiID: "picbi-42"}
	plain := &models.User{ID: uuid.New()}
	on := &fakeRemote{enabled: true}
	off := &fakeRemote{enabled: false}
	boom := errors.New("数据库炸了")

	cases := []struct {
		name       string
		user       *models.User
		remote     Remote
		localErr   error
		wantRemote bool
		wantErr    error
	}{
		// 本地扣成了就到此为止:关联过也不该多花一分钱。
		{"本地够用/已关联", linked, on, nil, false, nil},
		{"本地够用/未关联", plain, on, nil, false, nil},

		// 免费额度用完 —— 这才是 pic.bi 该接手的地方。
		{"月额度用完/已关联", linked, on, ErrNoCredits, true, nil},
		{"日限用完/已关联", linked, on, ErrDailyLimit, true, nil},

		// 没关联的人没有下一条路,而且报出去的必须还是本地那个错误:
		// 跟一个没绑过 pic.bi 的人说"pic.bi 不可用"毫无意义。
		{"月额度用完/未关联", plain, on, ErrNoCredits, false, ErrNoCredits},
		{"日限用完/未关联", plain, on, ErrDailyLimit, false, ErrDailyLimit},

		// 这个部署没配 partner 凭据,等于没接。
		{"月额度用完/已关联但未配置", linked, off, ErrNoCredits, false, ErrNoCredits},
		{"月额度用完/已关联但无实现", linked, nil, ErrNoCredits, false, ErrNoCredits},

		// 本地这一步出的不是"没钱",而是机器有问题。拿用户的钱去补是错的。
		{"本地报别的错/已关联", linked, on, boom, false, boom},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRemote, gotErr := chooseLedger(tc.user, tc.remote, tc.localErr)
			if gotRemote != tc.wantRemote {
				t.Errorf("走远程 = %v, 期望 %v", gotRemote, tc.wantRemote)
			}
			if tc.wantErr == nil && gotErr != nil {
				t.Errorf("错误 = %v, 期望 nil", gotErr)
			}
			if tc.wantErr != nil && !errors.Is(gotErr, tc.wantErr) {
				t.Errorf("错误 = %v, 期望 %v", gotErr, tc.wantErr)
			}
		})
	}
}

// 退款读的是记录上定格的账本,不是用户此刻的绑定状态。
//
// 这一条是整个改动里最贵的一个不变量:绑定 → 提交 → 立刻解绑 → 让生成失败,
// 如果退款去看"现在还绑着吗",pic.bi 的真钱就会被退成本地的免费次数,而解绑
// 一个 API token 就能调。函数签名里根本没有 *models.User 就是这个保证的形状,
// 这个测试盯的是它不被悄悄加回来。
func TestRefundFollowsFrozenLedger(t *testing.T) {
	t.Run("picbi 记录退到 pic.bi,按记录里的流水号", func(t *testing.T) {
		f := &fakeRemote{enabled: true}
		SetRemote(f)
		defer SetRemote(nil)

		var localCalls int
		defer swapRefundLocal(func(*gorm.DB, uuid.UUID, int) error {
			localCalls++
			return nil
		})()

		gen := &models.AIGeneration{
			ID:          uuid.New(),
			UserID:      uuid.New(),
			Ledger:      models.AIGenLedgerPicbi,
			PicbiUserID: "picbi-42",
			SpendOpID:   "op-777",
			Credits:     4,
		}
		// db 传 nil:远程这条路不该碰数据库,碰了就 panic,这本身就是断言。
		if err := Refund(nil, gen); err != nil {
			t.Fatalf("退款失败: %v", err)
		}
		if localCalls != 0 {
			t.Error("远程账本的记录不该退进本地余额")
		}
		if f.refundCalls != 1 {
			t.Fatalf("远程退款调用了 %d 次, 期望 1", f.refundCalls)
		}
		if f.lastRefundOp != "op-777" {
			t.Errorf("退的流水号 = %q, 期望记录里定格的 op-777", f.lastRefundOp)
		}
		// 幂等键由记录 ID 算出:重试算出来的是同一个键,pic.bi 才认得出
		// 这是同一笔退款而不是第二笔。
		if f.lastRefundKey != refundKey(gen.ID) {
			t.Errorf("幂等键 = %q, 期望 %q", f.lastRefundKey, refundKey(gen.ID))
		}
		if gen.RefundedAt == nil {
			t.Error("退成了却没盖章,对账器会再退一遍")
		}
	})

	t.Run("本地记录不碰 pic.bi,哪怕用户现在绑着", func(t *testing.T) {
		f := &fakeRemote{enabled: true}
		SetRemote(f)
		defer SetRemote(nil)

		var got struct {
			user    uuid.UUID
			credits int
			calls   int
		}
		defer swapRefundLocal(func(_ *gorm.DB, uid uuid.UUID, c int) error {
			got.user, got.credits, got.calls = uid, c, got.calls+1
			return nil
		})()

		uid := uuid.New()
		gen := &models.AIGeneration{
			ID:      uuid.New(),
			UserID:  uid,
			Ledger:  models.AIGenLedgerLocal,
			Credits: 1,
		}
		if err := Refund(nil, gen); err != nil {
			t.Fatalf("退款失败: %v", err)
		}
		if f.refundCalls != 0 || f.spendCalls != 0 {
			t.Error("本地账本的记录不该动 pic.bi")
		}
		if got.calls != 1 || got.user != uid || got.credits != 1 {
			t.Errorf("本地退款参数 = %+v, 期望 user=%s credits=1", got, uid)
		}
	})

	t.Run("存量记录(账本列为空)按本地退", func(t *testing.T) {
		f := &fakeRemote{enabled: true}
		SetRemote(f)
		defer SetRemote(nil)
		var calls int
		defer swapRefundLocal(func(*gorm.DB, uuid.UUID, int) error { calls++; return nil })()

		gen := &models.AIGeneration{ID: uuid.New(), UserID: uuid.New(), Credits: 1}
		if err := Refund(nil, gen); err != nil {
			t.Fatalf("退款失败: %v", err)
		}
		if calls != 1 || f.refundCalls != 0 {
			t.Error("空账本列的存量记录必须走本地")
		}
	})

	t.Run("已盖章的记录不再退第二次", func(t *testing.T) {
		f := &fakeRemote{enabled: true}
		SetRemote(f)
		defer SetRemote(nil)

		now := timeNow()
		gen := &models.AIGeneration{
			ID: uuid.New(), Ledger: models.AIGenLedgerPicbi,
			SpendOpID: "op-1", RefundedAt: &now,
		}
		if err := Refund(nil, gen); err != nil {
			t.Fatalf("退款失败: %v", err)
		}
		if f.refundCalls != 0 {
			t.Error("盖过章的记录被重复退款")
		}
	})
}

// 流水号丢了的记录:用同一个幂等键把它问回来,再退。
//
// 这条路对应的现场是"扣费请求发出去了,回执没回来"。当作没扣过的话,用户
// 充的钱就凭空消失且没有任何记录指得回去。
func TestRefundRecoversLostSpendOp(t *testing.T) {
	f := &fakeRemote{enabled: true, spendOpID: "op-recovered", spendCredit: 2}
	SetRemote(f)
	defer SetRemote(nil)

	gen := &models.AIGeneration{
		ID: uuid.New(), Ledger: models.AIGenLedgerPicbi,
		PicbiUserID: "picbi-42", Model: "gpt-image-2", Resolution: "2k",
	}
	if err := Refund(nil, gen); err != nil {
		t.Fatalf("退款失败: %v", err)
	}
	if f.spendCalls != 1 {
		t.Fatalf("重放 spend 调用了 %d 次, 期望 1", f.spendCalls)
	}
	if f.lastRefundOp != "op-recovered" {
		t.Errorf("退的流水号 = %q, 期望问回来的 op-recovered", f.lastRefundOp)
	}

	// 对端说"这个人余额不足"——既然现在都扣不动,当初那次自然也没扣成。
	// 没有东西可退,而且绝不能改退本地。
	f2 := &fakeRemote{enabled: true, spendErr: &kindErr{kind: "no_credits"}}
	SetRemote(f2)
	gen2 := &models.AIGeneration{ID: uuid.New(), Ledger: models.AIGenLedgerPicbi, PicbiUserID: "picbi-42"}
	if err := Refund(nil, gen2); err != nil {
		t.Fatalf("确认没扣过的记录不该报错: %v", err)
	}
	if f2.refundCalls != 0 {
		t.Error("没扣成的记录不该发起退款")
	}
}

// 认不出来的远程错误一律当"不可达"。不确定的时候按最保守的处理:挡住,
// 而不是放行一次不花钱的生成。
func TestMapRemoteErr(t *testing.T) {
	cases := map[string]error{
		"no_credits":  ErrNoCredits,
		"forbidden":   ErrRemoteDenied,
		"invalid":     ErrRemoteDenied,
		"conflict":    ErrRemoteDenied,
		"unavailable": ErrRemoteUnavailable,
		"什么鬼":         ErrRemoteUnavailable,
	}
	for kind, want := range cases {
		if got := mapRemoteErr(&kindErr{kind: kind}); !errors.Is(got, want) {
			t.Errorf("%s → %v, 期望 %v", kind, got, want)
		}
	}
	if got := mapRemoteErr(errors.New("裸错误")); !errors.Is(got, ErrRemoteUnavailable) {
		t.Errorf("裸错误 → %v, 期望 ErrRemoteUnavailable", got)
	}
	if mapRemoteErr(nil) != nil {
		t.Error("nil 该原样返回 nil")
	}
}

func TestDefinitelyNotCharged(t *testing.T) {
	for _, kind := range []string{"no_credits", "forbidden", "invalid"} {
		if !definitelyNotCharged(&kindErr{kind: kind}) {
			t.Errorf("%s 是对端明确拒绝,应当判定为没扣", kind)
		}
	}
	// 结果未知的一律按"可能扣了"处理:多退一次远比少退一次便宜。
	for _, kind := range []string{"unavailable", "conflict", "没见过"} {
		if definitelyNotCharged(&kindErr{kind: kind}) {
			t.Errorf("%s 结果未知,不该判定为没扣", kind)
		}
	}
	if definitelyNotCharged(errors.New("超时")) {
		t.Error("裸错误结果未知,不该判定为没扣")
	}
}

// swapRefundLocal 换掉本地退款的实现,返回还原用的函数。
func swapRefundLocal(fn func(*gorm.DB, uuid.UUID, int) error) func() {
	prev := refundLocal
	refundLocal = fn
	return func() { refundLocal = prev }
}
