package promote

import (
	"testing"

	"github.com/gentpan/openimg/backend/internal/models"
)

func TestUsageRatio(t *testing.T) {
	cases := []struct {
		quota, used int64
		want        float64
	}{
		{1000, 700, 0.7},
		{1000, 0, 0},
		{1000, 1000, 1},
		// 配额为 0 时不能除——真实数据里存在这种行(未发放额度的新账号),
		// 除下去是 +Inf,而 +Inf >= NeedUsageRatio 恒真,等于给每个零配额
		// 账号自动升级。
		{0, 100, 0},
		{0, 0, 0},
	}
	for _, c := range cases {
		got := usageRatio(&models.User{QuotaBytes: c.quota, UsedBytes: c.used})
		if got != c.want {
			t.Errorf("usageRatio(quota=%d, used=%d) = %v, want %v", c.quota, c.used, got, c.want)
		}
	}
}

// 门槛之间要互相说得通。这个测试是给以后调数值的人看的:随手改一个常量而
// 让整套规则失去意义时,它会先响。
func TestThresholdsAreCoherent(t *testing.T) {
	if MinUploadDays < 2 {
		t.Error("上传天数门槛低于 2 天就挡不住一次性刷量,那是这条规则存在的唯一理由")
	}
	if NeedUsageRatio <= 0 || NeedUsageRatio > 1 {
		t.Errorf("NeedUsageRatio = %v,应落在 (0, 1]", NeedUsageRatio)
	}
	if LoyaltyUsageRatio < NeedUsageRatio {
		t.Errorf("扩容的用量门槛(%v)不该低于升级的(%v)——扩容是更贵的奖励,"+
			"门槛反而更松说不通", LoyaltyUsageRatio, NeedUsageRatio)
	}
	// trusted 注册额度是 10G。上限 40G 意味着最多翻两次,再往上就停。
	const trustedBase = 10 * gib
	if LoyaltyCap <= trustedBase {
		t.Errorf("LoyaltyCap = %d 不高于 trusted 基础额度 %d,扩容永远发不出去",
			LoyaltyCap, trustedBase)
	}
	if LoyaltyCap > trustedBase*8 {
		t.Errorf("LoyaltyCap = %d 超过基础额度 8 倍,翻倍复利会跑飞", LoyaltyCap)
	}
	if LoyaltyMinActiveMonths > 6 {
		t.Error("半年里要求超过 6 个月有上传是不可能满足的")
	}
	if LoyaltyMinActiveMonths < 2 {
		t.Error("低于 2 个月等于不看持续性,那就只是在奖励用量")
	}
}

func TestResultAny(t *testing.T) {
	if (Result{}).Any() {
		t.Error("空结果不该报告发生了什么")
	}
	if !(Result{Promoted: true}).Any() {
		t.Error("升级了要报告")
	}
	if !(Result{Loyalty: true}).Any() {
		t.Error("扩容了要报告")
	}
	// 只有字节数、两个标志都没立起来,说明调用方漏设了标志。
	if (Result{GrantedBytes: 1024}).Any() {
		t.Error("光有字节数不算发生过什么,标志才是判据")
	}
}
