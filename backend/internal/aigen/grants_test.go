package aigen

import (
	"testing"
	"time"
)

func TestStreakReward(t *testing.T) {
	cases := []struct {
		streak  int
		credits int
		label   string
	}{
		{1, 0, ""}, {9, 0, ""},
		{10, 3, "streak_10"},
		{11, 0, ""}, {19, 0, ""},
		{20, 5, "streak_20"},
		{30, 10, "streak_30"},
		{31, 0, ""}, {59, 0, ""},
		{60, 20, "streak_60"},
		// 走完阶梯后每 30 天一次,不是每天都有。
		{61, 0, ""}, {89, 0, ""},
		{90, 10, "streak_90"},
		{91, 0, ""},
		{120, 10, "streak_120"},
		{150, 10, "streak_150"},
		// 40 是 10 的倍数但不是里程碑,也不在 60 之后的循环上。
		{40, 0, ""}, {50, 0, ""},
	}
	for _, c := range cases {
		gotC, gotL := streakReward(c.streak)
		if gotC != c.credits || gotL != c.label {
			t.Errorf("streakReward(%d) = (%d, %q), want (%d, %q)",
				c.streak, gotC, gotL, c.credits, c.label)
		}
	}
}

// 里程碑只在"恰好等于"那天给,不是"大于等于"。写成 >= 的话每天都会重复发,
// 连签 100 天能把 60 天那笔领 41 次。
func TestStreakRewardFiresOnceNotEveryDayAfter(t *testing.T) {
	hits := 0
	for s := 1; s <= 89; s++ {
		if c, _ := streakReward(s); c > 0 {
			hits++
		}
	}
	if hits != 4 { // 10 / 20 / 30 / 60
		t.Errorf("前 89 天应恰好命中 4 次里程碑,实际 %d 次", hits)
	}
}

func TestMonthEnd(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2026-08-22T13:04:05Z", "2026-09-01T00:00:00Z"},
		{"2026-08-01T00:00:00Z", "2026-09-01T00:00:00Z"},
		// 跨年:month+1 = 13,靠 time.Date 自己进位。
		{"2026-12-15T09:00:00Z", "2027-01-01T00:00:00Z"},
		{"2026-12-31T23:59:59Z", "2027-01-01T00:00:00Z"},
		// 闰年二月。
		{"2028-02-29T12:00:00Z", "2028-03-01T00:00:00Z"},
	}
	for _, c := range cases {
		in, err := time.Parse(time.RFC3339, c.in)
		if err != nil {
			t.Fatal(err)
		}
		got := monthEnd(in).UTC().Format(time.RFC3339)
		if got != c.want {
			t.Errorf("monthEnd(%s) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestSameDay(t *testing.T) {
	p := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if !sameDay(p("2026-08-22T00:00:01Z"), p("2026-08-22T23:59:59Z")) {
		t.Error("同一天的两个时刻应判为同一天")
	}
	if sameDay(p("2026-08-22T23:59:59Z"), p("2026-08-23T00:00:01Z")) {
		t.Error("跨过零点应判为不同天")
	}
}

// 满勤 60 天能攒出多少,决定 MaxGrantedCredits 该设多大。这个测试会在有人
// 调高里程碑却忘了调上限时失败。
func TestMaxGrantedCoversFullAttendance(t *testing.T) {
	peak := 60 // 日签每天 1 次
	for s := 1; s <= 60; s++ {
		c, _ := streakReward(s)
		peak += c
	}
	if peak > MaxGrantedCredits {
		t.Errorf("满勤 60 天可攒 %d 次,已超过上限 %d——上限会把正常用户挡住",
			peak, MaxGrantedCredits)
	}
	if MaxGrantedCredits > peak*2 {
		t.Errorf("上限 %d 相对满勤峰值 %d 过于宽松,失去了防刷意义",
			MaxGrantedCredits, peak)
	}
}
