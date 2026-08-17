package aigen

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNoCredits  = errors.New("aigen: 生成次数已用完")
	ErrDailyLimit = errors.New("aigen: 今日生成次数已达上限")
)

// Balance 是一个用户当下能用多少次的完整答案。
type Balance struct {
	Credits    int `json:"credits"`
	UsedToday  int `json:"used_today"`
	DailyLimit int `json:"daily_limit"`
	Monthly    int `json:"monthly"`
}

// Remaining 是这一刻真正还能生成几次:余额与今日剩余里更小的那个。
func (b Balance) Remaining() int {
	left := b.DailyLimit - b.UsedToday
	if left < 0 {
		left = 0
	}
	if b.Credits < left {
		return b.Credits
	}
	return left
}

func monthKey(t time.Time) string { return t.UTC().Format("2006-01") }

// EnsureMonthly 按需补发当月额度。
//
// 没有定时任务:这个项目刻意不跑 cron(见 scheduler 的注释),所以月度发放
// 在"用到的时候"结算——发现记录里的月份不是本月,就把余额重置为本月的量。
// 重置而不是累加:每月 50 张是"每月的配给"而不是"攒着不过期的存货",否则
// 半年不用的账号会攒出 300 张的突袭额度。签到送的次数在本月内有效,同理。
func EnsureMonthly(db *gorm.DB, u *models.User, g models.UserGroup) error {
	now := monthKey(time.Now())
	if u.AICreditsMonth == now {
		return nil
	}
	u.AICredits = g.AIMonthly
	u.AICreditsMonth = now
	return db.Model(&models.User{}).Where("id = ?", u.ID).
		Updates(map[string]any{
			"ai_credits":       u.AICredits,
			"ai_credits_month": u.AICreditsMonth,
		}).Error
}

// UsedToday 数今天已经提交过几次。
//
// 按提交计而不是按成功计:上游那一次调用无论成没成都花掉了,而且按成功计
// 的话,一个反复失败的提示词可以无限次撞上游。失败会退还余额(见 Refund),
// 但今日次数不退——这正是它作为"闸门"的意义。
func UsedToday(db *gorm.DB, userID uuid.UUID) (int, error) {
	start := time.Now().UTC().Truncate(24 * time.Hour)
	var n int64
	err := db.Model(&models.AIGeneration{}).
		Where("user_id = ? AND created_at >= ?", userID, start).
		Count(&n).Error
	return int(n), err
}

func Current(db *gorm.DB, u *models.User, g models.UserGroup) (Balance, error) {
	if err := EnsureMonthly(db, u, g); err != nil {
		return Balance{}, err
	}
	used, err := UsedToday(db, u.ID)
	if err != nil {
		return Balance{}, err
	}
	return Balance{
		Credits:    u.AICredits,
		UsedToday:  used,
		DailyLimit: g.AIDaily,
		Monthly:    g.AIMonthly,
	}, nil
}

// Reserve 在提交给上游之前先把次数扣掉。
//
// 先扣后用:上游那一次调用真的花了钱,先调再扣会让并发请求越过余额。扣不
// 动就直接拒,不进上游。
func Reserve(db *gorm.DB, u *models.User, g models.UserGroup, credits int) error {
	if credits <= 0 {
		credits = 1
	}
	if err := EnsureMonthly(db, u, g); err != nil {
		return err
	}
	used, err := UsedToday(db, u.ID)
	if err != nil {
		return err
	}
	if g.AIDaily > 0 && used >= g.AIDaily {
		return ErrDailyLimit
	}

	// 条件更新兼作并发闸门:两个请求同时进来,只有一个能把余额从 n 减到
	// n-1,另一个 RowsAffected 为 0。
	res := db.Model(&models.User{}).
		Where("id = ? AND ai_credits >= ?", u.ID, credits).
		UpdateColumn("ai_credits", gorm.Expr("ai_credits - ?", credits))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNoCredits
	}
	u.AICredits -= credits
	return nil
}

// Begin 开一次生成:在同一个事务里数今日、扣额度、落记录。
//
// 这三步必须原子,否则日限拦不住任何东西。UsedToday 数的是 ai_generations
// 的行数,而这行原本要等上游递交成功才写——中间隔着一次几十秒的 HTTP。同时
// 提交的请求会全部读到同一个旧计数、全部通过检查,日限形同虚设(只剩月额度
// 那个条件更新兜底,后果从"今天 5 张"退化成"一次烧光整月")。
//
// 用用户行的排他锁做串行点:粒度正好是"同一个用户的并发提交",既不会拖累
// 别人,也不需要 INSERT...SELECT 那种读起来费劲的条件插入。
//
// gen 需预先填好 Prompt/Model/Size/Resolution/Credits;返回时它已入库,状态
// 为 charging。
func Begin(db *gorm.DB, u *models.User, g models.UserGroup, gen *models.AIGeneration) error {
	if gen.Credits <= 0 {
		gen.Credits = 1
	}
	return db.Transaction(func(tx *gorm.DB) error {
		// 锁住用户行。之后的数数与扣减都在这把锁里,同一用户的第二个请求
		// 会等在这里,拿到的是已经包含前一次的计数。
		var locked models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&locked, "id = ?", u.ID).Error; err != nil {
			return err
		}
		if err := ensureMonthlyTx(tx, &locked, g); err != nil {
			return err
		}

		if g.AIDaily > 0 {
			start := time.Now().UTC().Truncate(24 * time.Hour)
			var used int64
			if err := tx.Model(&models.AIGeneration{}).
				Where("user_id = ? AND created_at >= ?", u.ID, start).
				Count(&used).Error; err != nil {
				return err
			}
			if int(used) >= g.AIDaily {
				return ErrDailyLimit
			}
		}

		if locked.AICredits < gen.Credits {
			return ErrNoCredits
		}
		if err := tx.Model(&models.User{}).Where("id = ?", u.ID).
			UpdateColumn("ai_credits", gorm.Expr("ai_credits - ?", gen.Credits)).Error; err != nil {
			return err
		}

		gen.UserID = u.ID
		gen.Status = models.AIGenCharging
		if err := tx.Create(gen).Error; err != nil {
			return err
		}

		// 事务提交后调用方手里的 u 才该反映新余额。
		u.AICredits = locked.AICredits - gen.Credits
		u.AICreditsMonth = locked.AICreditsMonth
		return nil
	})
}

// ensureMonthlyTx 是 EnsureMonthly 的事务内版本。逻辑一致,只是复用外面
// 传进来的 tx,好让补发与扣减落在同一把锁里。
func ensureMonthlyTx(tx *gorm.DB, u *models.User, g models.UserGroup) error {
	now := monthKey(time.Now())
	if u.AICreditsMonth == now {
		return nil
	}
	u.AICredits = g.AIMonthly
	u.AICreditsMonth = now
	return tx.Model(&models.User{}).Where("id = ?", u.ID).
		Updates(map[string]any{
			"ai_credits":       u.AICredits,
			"ai_credits_month": u.AICreditsMonth,
		}).Error
}

// Refund 在生成失败时把次数还回去。今日次数不退,理由见 UsedToday。
func Refund(db *gorm.DB, userID uuid.UUID, credits int) error {
	if credits <= 0 {
		return nil
	}
	return db.Model(&models.User{}).Where("id = ?", userID).
		UpdateColumn("ai_credits", gorm.Expr("ai_credits + ?", credits)).Error
}

// GrantCheckin 是签到附带的赠送:随机 [min, max] 次。
//
// 返回实际送出的次数,好让签到接口能如实告诉用户拿到了什么。上限按当月额度
// 的两倍封顶——签到是锦上添花,不该让人靠天天签到把月配额顶到十倍。
func GrantCheckin(db *gorm.DB, u *models.User, g models.UserGroup) (int, error) {
	if g.AICheckinMax <= 0 {
		return 0, nil
	}
	if err := EnsureMonthly(db, u, g); err != nil {
		return 0, err
	}
	lo, hi := g.AICheckinMin, g.AICheckinMax
	if lo < 0 {
		lo = 0
	}
	if hi < lo {
		hi = lo
	}
	want := lo
	if hi > lo {
		want = lo + rand.IntN(hi-lo+1)
	}
	if want == 0 {
		return 0, nil
	}
	cap := g.AIMonthly * 2
	if cap > 0 && u.AICredits+want > cap {
		want = cap - u.AICredits
	}
	if want <= 0 {
		return 0, nil
	}
	if err := db.Model(&models.User{}).Where("id = ?", u.ID).
		UpdateColumn("ai_credits", gorm.Expr("ai_credits + ?", want)).Error; err != nil {
		return 0, err
	}
	u.AICredits += want
	return want, nil
}

// Describe 供日志与账本文案使用。
func Describe(prompt string) string {
	return fmt.Sprintf("AI 生成：%s", truncate(prompt, 60))
}
