package api

import (
	"context"
	"log"
	"time"

	"github.com/gentpan/openimg/backend/internal/aigen"
	"github.com/gentpan/openimg/backend/internal/i18n"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/scheduler"
)

// AI 生成的对账窗口。
//
// 一次生成通常几十秒。三个数分开的理由:
//   - aiStale 是"这条记录看起来卡住了"的门槛。取 15 分钟,远大于正常耗时,
//     也远大于提交那一步 30 秒的超时,所以正在处理中的记录不会被误判。
//   - aiGiveUp 是"别再等了"。超过它就终结并退款——上游真的丢了任务时,
//     一直挂着不如把额度还给用户。
//   - aiSweepEvery 是巡检间隔。做不到即时,但对账本来就是兜底,不是主路径。
const (
	aiStale      = 15 * time.Minute
	aiGiveUp     = time.Hour
	aiSweepEvery = 10 * time.Minute
	aiSweepLimit = 500
	// 退款重试的窗口与批量。七天之外就别再敲了:那时候要么是对端账本出了
	// 需要人介入的问题,要么是这条记录本来就退不了,继续每十分钟打一次
	// 只会把日志淹掉。
	aiRefundRetryWindow = 7 * 24 * time.Hour
	aiRefundRetryLimit  = 200
)

// StartAIReconciler 定期回收卡住的生成记录。
//
// 光靠启动时扫一遍不够。队列是有意做成会丢的(Submit 满了就丢,见 scheduler
// 的注释),重试次数用尽也只打一行日志——这两种情况下进程根本没重启,启动
// 扫描永远等不到。而每一条卡住的记录背后都是已经扣掉的额度:不回收,用户
// 就是白扣。
func (s *Server) StartAIReconciler(ctx context.Context) {
	if s.Queue == nil || s.AIGen == nil || !s.AIGen.Enabled() {
		return
	}
	go func() {
		t := time.NewTicker(aiSweepEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.reconcileAIGenerations(ctx)
				s.retryFailedRefunds(ctx)
			}
		}
	}()
}

// reconcileAIGenerations 扫一遍非终态的老记录,该重投的重投,该了结的了结。
func (s *Server) reconcileAIGenerations(ctx context.Context) {
	var stuck []models.AIGeneration
	if err := s.DB.
		Where("status NOT IN ? AND updated_at < ?",
			[]models.AIGenStatus{models.AIGenCompleted, models.AIGenFailed},
			time.Now().Add(-aiStale)).
		Limit(aiSweepLimit).Find(&stuck).Error; err != nil {
		log.Printf("ai: 对账查询失败: %v", err)
		return
	}

	var requeued, closed int
	for i := range stuck {
		gen := &stuck[i]
		switch {
		// 没有任务号说明扣了费却没递交出去(进程在这两步之间死了,或者提交
		// 那一步报错后连状态都没写成)。上游没有东西可查,直接退款了结。
		//
		// 走 pic.bi 的记录也落在这一支:beginRemote 在"扣费结果未知"时刻意
		// 把行留在 charging,等的就是这里。failAIGen 里的退款会用同一个幂等
		// 键把丢掉的流水号问回来,再原路退。
		case gen.TaskID == "":
			if err := s.failAIGen(gen, i18n.TCtx(ctx, "ai.abandoned")); err != nil {
				log.Printf("ai: 了结 %s 失败: %v", gen.ID, err)
				continue
			}
			closed++
		// 太久了。上游可能真把任务弄丢了,继续等下去只是让额度一直悬着。
		case time.Since(gen.CreatedAt) > aiGiveUp:
			if err := s.failAIGen(gen, i18n.TCtx(ctx, "ai.timed_out")); err != nil {
				log.Printf("ai: 了结 %s 失败: %v", gen.ID, err)
				continue
			}
			closed++
		default:
			// 还在窗口内,重新入队接着轮。这里用 SubmitWait 而不是 Submit:
			// 对账本身就是来兜住"任务被丢掉"的,再丢一次就没有下一道防线了。
			if err := s.Queue.SubmitWait(ctx, scheduler.JobAIPoll, gen.ID); err != nil {
				return // ctx 结束,剩下的下一轮再说
			}
			requeued++
		}
	}
	if requeued+closed > 0 {
		log.Printf("ai: 对账重投 %d 条、了结 %d 条", requeued, closed)
	}
}

// retryFailedRefunds 把没退成的远程扣费再退一遍。
//
// 只针对 pic.bi 账本:本地退款是一条几乎不会失败的 UPDATE,而远程退款隔着
// 一次跨服务调用,网络抖一下就会失败——而记录那时已经是 failed 终态,再没有
// 任何路径会回来看它。少了这一趟,用户充值的真钱就停在对端不回来了,现场
// 只剩一行日志。
//
// 重试是安全的:退款的幂等键由记录 ID 算出,同一笔退两次在 pic.bi 那边只
// 执行一次;而 RefundedAt 一旦盖上,这里就再也选不中它。
func (s *Server) retryFailedRefunds(ctx context.Context) {
	var rows []models.AIGeneration
	if err := s.DB.
		Where("status = ? AND ledger = ? AND refunded_at IS NULL AND created_at > ?",
			models.AIGenFailed, models.AIGenLedgerPicbi, time.Now().Add(-aiRefundRetryWindow)).
		Limit(aiRefundRetryLimit).Find(&rows).Error; err != nil {
		log.Printf("ai: 退款重试查询失败: %v", err)
		return
	}
	var ok int
	for i := range rows {
		select {
		case <-ctx.Done():
			return
		default:
		}
		gen := &rows[i]
		if err := aigen.Refund(s.DB, gen); err != nil {
			log.Printf("ai: 退款重试仍失败 gen=%s user=%s picbi_user=%s op=%s credits=%d: %v",
				gen.ID, gen.UserID, gen.PicbiUserID, gen.SpendOpID, gen.Credits, err)
			continue
		}
		ok++
	}
	if ok > 0 {
		log.Printf("ai: 补退 %d 条 pic.bi 扣费", ok)
	}
}
