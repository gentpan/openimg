package api

import "testing"

// 天数要夹住。放开上限没意义——再长的窗口一屏也画不下,而每多一天就多一行要
// 扫的记录;放开下限则会画出一张只有两三个点的"折线"。
func TestUploadTrendDaysClamp(t *testing.T) {
	for _, c := range []struct {
		in   int
		want int
	}{
		{0, 7}, {1, 7}, {6, 7}, {7, 7},
		{10, 10}, {30, 30}, {90, 90},
		{91, 90}, {3650, 90}, {-5, 7},
	} {
		got := c.in
		if got < 7 {
			got = 7
		}
		if got > 90 {
			got = 90
		}
		if got != c.want {
			t.Errorf("days=%d 夹成 %d,想要 %d", c.in, got, c.want)
		}
	}
}
