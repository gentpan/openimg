package api

import (
	"context"
	"log"
	"time"

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
