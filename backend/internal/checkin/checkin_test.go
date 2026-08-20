package checkin

import (
	"testing"
	"time"

	"github.com/gentpan/openimg/backend/internal/models"
)

// 日界按用户时区算,这是这次改动的全部意义:UTC 下东八区用户的"一天"从早上
// 8 点开始,而那不是他实际过的那一天。
func TestTodayInFollowsTheUsersZone(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("加载时区失败: %v", err)
	}
	now := time.Now()
	if got, want := TodayIn(shanghai), now.In(shanghai).Format("2006-01-02"); got != want {
		t.Errorf("上海时区下的今天 = %s, 想要 %s", got, want)
	}
	if got, want := TodayIn(time.UTC), now.UTC().Format("2006-01-02"); got != want {
		t.Errorf("UTC 下的今天 = %s, 想要 %s", got, want)
	}
	// nil 退回 UTC,而不是崩。老调用点传得进 nil。
	if TodayIn(nil) != TodayIn(time.UTC) {
		t.Error("nil 时区应当退回 UTC")
	}
}

// 没填时区的老账号必须还是 UTC —— 这个字段加进来之前就是这个行为,不能因为
// 一次升级把所有人的日界挪动几个小时。
func TestEmptyTimezoneStaysUTC(t *testing.T) {
	var u models.User
	if u.Location() != time.UTC {
		t.Error("空时区应当是 UTC")
	}
	u.Timezone = "Nowhere/Nothing"
	if u.Location() != time.UTC {
		t.Error("认不出的时区应当退回 UTC,而不是崩或者用别的")
	}
	u.Timezone = "Asia/Shanghai"
	if u.Location().String() != "Asia/Shanghai" {
		t.Errorf("认得出的时区应当生效,得到 %s", u.Location())
	}
}

// 按本地时区算日界之后多出来的一个洞:签完之后把时区往西调,"今天"会退回昨
// 天。等值判断在那时不成立,于是能再签一次。日期是 ISO 串,字典序就是时间序,
// 所以判定用 >= 而不是 ==。
func TestWesternShiftCannotBuyASecondCheckin(t *testing.T) {
	// 用户在 UTC+14 签了 8-21,然后把时区改到 UTC-11,那边还是 8-20。
	east, _ := time.LoadLocation("Pacific/Kiritimati") // UTC+14
	west, _ := time.LoadLocation("Pacific/Midway")     // UTC-11
	if east == nil || west == nil {
		t.Skip("这台机器上没有时区库")
	}

	signed := TodayIn(east)
	nowThere := TodayIn(west)
	if signed <= nowThere {
		t.Skipf("此刻两地同一天(%s / %s),这条测试要跨日才有意义", signed, nowThere)
	}

	// 等值判断会放行 —— 这正是不能用 == 的原因。
	if signed == nowThere {
		t.Fatal("构造失败:两个日期应当不同")
	}
	// 现在的判定是 >=,挡住。
	if !(signed >= nowThere) {
		t.Errorf("已签 %s 的账号在 %s 应当被挡住", signed, nowThere)
	}
}

func TestYesterdayOf(t *testing.T) {
	if got := yesterdayOf("2026-01-01"); got != "2025-12-31" {
		t.Errorf("跨年往回一天 = %s, 想要 2025-12-31", got)
	}
	if got := yesterdayOf("2026-03-01"); got != "2026-02-28" {
		t.Errorf("非闰年 3-1 往回 = %s, 想要 2026-02-28", got)
	}
	if got := yesterdayOf("不是日期"); got != "" {
		t.Errorf("解不出的日期应当返回空,得到 %q", got)
	}
}
