package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

// GET /api/stats/uploads?days=30 — 按天的上传条数与字节。
//
// 客户端原来是拿"图库当前那一页"按天聚合出趋势图的。那份数据受排序和搜索影响:
// 把排序切成「占用最大」再看趋势,画出来的图**不是不全,是彻底错的**;而想画 30
// 天更不可能——一页只有几十张。
//
// 日界按用户所在时区算,与签到同一套口径(见 checkin.TodayIn)。用 UTC 的话东八区
// 用户会看到"今天"从早上 8 点开始,凌晨传的图落到前一天的柱子上。
type uploadTrendPoint struct {
	Date  string `json:"date"` // YYYY-MM-DD,用户所在时区
	Count int    `json:"count"`
	Bytes int64  `json:"bytes"`
}

func (s *Server) handleUploadTrend(c *gin.Context) {
	u := auth.MustUser(c)

	// 天数夹在 7…90。放开上限没有意义:再长的窗口一屏也画不下,而每多一天就多
	// 一行要扫的记录。
	days := 30
	if v := c.Query("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			days = n
		}
	}
	if days < 7 {
		days = 7
	}
	if days > 90 {
		days = 90
	}

	loc := u.Location()
	// 从今天往前数 days-1 天,包含今天。
	today := time.Now().In(loc)
	start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc).
		AddDate(0, 0, -(days - 1))

	// 在 SQL 里按用户时区分组。AT TIME ZONE 收 IANA 名,而 Location() 保证了它
	// 要么是库里认得的名字、要么是 UTC。
	zone := loc.String()
	type row struct {
		Day   time.Time
		Count int
		Bytes int64
	}
	var rows []row
	err := s.DB.Raw(`
		SELECT date_trunc('day', created_at AT TIME ZONE ?) AS day,
		       COUNT(*) AS count,
		       COALESCE(SUM(size_stored), 0) AS bytes
		  FROM images
		 WHERE user_id = ? AND deleted_at IS NULL AND created_at >= ?
		 GROUP BY day
		 ORDER BY day`,
		zone, u.ID, start.UTC()).Scan(&rows).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	byDay := make(map[string]row, len(rows))
	for _, r := range rows {
		byDay[r.Day.Format("2006-01-02")] = r
	}

	// 补齐没有上传的那些天。缺口留着不填的话,客户端画出来的折线会把两个相隔
	// 一周的点直接连起来,看着像那一周一直在稳定上传。
	out := make([]uploadTrendPoint, 0, days)
	for i := 0; i < days; i++ {
		key := start.AddDate(0, 0, i).Format("2006-01-02")
		r := byDay[key]
		out = append(out, uploadTrendPoint{Date: key, Count: r.Count, Bytes: r.Bytes})
	}
	// 明确禁缓存。
	//
	// 这份数据每传一张图就变,而且只有一两千字节——缓存它换不来什么,却会引入
	// 一个很难查的失败:中间层给响应挂上校验器之后,客户端下次带条件请求过来会
	// 拿到 **304 和空 body**。URLSession 只有在本地缓存还留着那条记录时才补得
	// 上内容,补不上就是空的,而解码器只接受 2xx —— 表现是趋势图整块消失,没有
	// 任何报错。
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"days": days, "points": out})
}
