package api

import (
	"context"
	"log"
	"time"

	"github.com/gentpan/openimg/backend/internal/email"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/storage"
)

const (
	// inactiveAfter 多久没露面才算沉寂。
	inactiveAfter = 180 * 24 * time.Hour

	// inactiveSweepEvery 扫描频率。一天一次绰绰有余——判据是以半年为单位
	// 变化的,扫得再勤也只是把同一批人反复查一遍。
	inactiveSweepEvery = 24 * time.Hour

	// inactiveBatch 单轮最多发几封。
	//
	// 有上限是因为第一次跑会撞上"所有历史沉寂用户"这个存量:上线当天可能
	// 有几千个符合条件,一次性发出去既会撞上游的速率限制,也会让这批信被
	// 收件方当成垃圾邮件。分几天慢慢发,对谁都好。
	inactiveBatch = 50
)

// StartInactiveNotifier 起一个每天一次的扫描,给沉寂用户发"你的图还在"。
//
// 没配邮件就不起——这功能唯一的动作就是发信,发不出去时空转没有意义。
func (s *Server) StartInactiveNotifier(ctx context.Context) {
	if s.Email == nil || !s.Email.Configured() {
		log.Printf("inactive: 邮件未配置,沉寂提醒不启用")
		return
	}
	// 打一行启动日志。这个功能常态下的正确行为就是"什么都不做"(没人符合
	// 条件时它连日志都不打),没有这行就分不清"跑了但没人符合"和"根本没起来"。
	log.Printf("inactive: 沉寂提醒已启用（%.0f 天未登录，每轮最多 %d 封）",
		inactiveAfter.Hours()/24, inactiveBatch)
	go func() {
		// 启动后先跑一轮,而不是干等满一个周期。
		//
		// 只靠 24 小时的 ticker 有个隐患:部署会重启进程,而活跃开发期一天
		// 可能部署好几次,于是计时器永远走不到头,这件事就永远不发生。延迟
		// 十分钟是为了避开启动那阵子的忙乱,多跑几轮也无妨——判重条件保证
		// 同一个人在同一轮沉寂里只会被打扰一次。
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Minute):
		}
		s.notifyInactive(ctx)

		t := time.NewTicker(inactiveSweepEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.notifyInactive(ctx)
			}
		}
	}()
}

// notifyInactive 找出这一轮该提醒的人并发信。
func (s *Server) notifyInactive(ctx context.Context) {
	cutoff := time.Now().Add(-inactiveAfter)

	var users []models.User
	// COALESCE 到 created_at:注册完再没登录过的账号 last_login_at 是空的,
	// 拿 NULL 去比较会把他们整个漏掉,而那恰恰是最沉寂的一批。
	//
	// 判重比的是两个时间戳的先后:提醒过之后如果他回来登录了,last_login_at
	// 会往前走,于是下一轮沉寂满半年时又能再提醒一次。
	//
	// 没有图的账号不打扰——这封信的全部内容就是"你的图还在",没有图就无话可说。
	err := s.DB.Where(`
		COALESCE(last_login_at, created_at) < ?
		AND (inactive_notified_at IS NULL OR inactive_notified_at < COALESCE(last_login_at, created_at))
		AND status = ?
		AND email_verified = true
		AND email_notify = true
		AND EXISTS (SELECT 1 FROM images WHERE images.user_id = users.id AND images.deleted_at IS NULL)
	`, cutoff, models.UserActive).
		Order("COALESCE(last_login_at, created_at) ASC").
		Limit(inactiveBatch).Find(&users).Error
	if err != nil {
		log.Printf("inactive: 查询沉寂用户失败: %v", err)
		return
	}
	if len(users) == 0 {
		return
	}

	site := s.PublicBaseURL
	sent := 0
	for i := range users {
		u := &users[i]
		var n int64
		if err := s.DB.Model(&models.Image{}).
			Where("user_id = ? AND deleted_at IS NULL", u.ID).Count(&n).Error; err != nil {
			log.Printf("inactive: 数图片失败 user=%s: %v", u.ID, err)
			continue
		}
		body := email.InactiveEmailHTML(
			email.MailBrand(""), u.Name, int(n),
			humanBytes(u.UsedBytes),
			storage.JoinURL(site, "gallery"),
		)
		sendCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		err := s.Email.Send(sendCtx, u.Email, "好久不见 —— 你在 Openimg 的图都还在", body)
		cancel()
		if err != nil {
			// 不盖章,下一轮还会选中他重试。发信失败是暂时的,而漏掉一个人
			// 要等半年才有下次机会。
			log.Printf("inactive: 发信失败 user=%s: %v", u.ID, err)
			continue
		}
		now := time.Now()
		if err := s.DB.Model(&models.User{}).Where("id = ?", u.ID).
			UpdateColumn("inactive_notified_at", &now).Error; err != nil {
			log.Printf("inactive: 盖章失败 user=%s: %v", u.ID, err)
		}
		sent++
	}
	log.Printf("inactive: 本轮提醒 %d/%d 个沉寂账号", sent, len(users))
}
