package aigen

import (
	"fmt"
	"log"
	"time"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GrantTTL 是签到赠送的有效期。
//
// 有效期不是为了省钱——没用掉的额度本来就不产生成本,上游只在真的生成时收费。
// 它防的是另一件事:余额无限累积。日签每天 +1、连签里程碑再叠上去,一个常年
// 签到的账号两年能攒出上千次,而这笔债随时可能被一次性兑现。
const GrantTTL = 60 * 24 * time.Hour

// MaxGrantedCredits 是赠送额度的上限,只管签到那部分,不含每月配给。
//
// 满勤 60 天理论峰值 98 次(日签 60 + 里程碑 38),留一截余量。它防的不是正常
// 用户——是脚本、是某天把签到接口刷爆的那种账号。
const MaxGrantedCredits = 150

// StreakReward 是一级连签里程碑。
type StreakReward struct {
	Days    int
	Credits int
}

// StreakRewards 是里程碑阶梯,按天数升序。
//
// 给得起是因为几乎没人拿得到:能连签 30 天的比例在 5% 上下,60 天不到 1%。
// 真正的成本大头是每日那 +1(占八成以上),里程碑既便宜又是最强的坚持理由。
var StreakRewards = []StreakReward{
	{Days: 10, Credits: 3},
	{Days: 20, Credits: 5},
	{Days: 30, Credits: 10},
	{Days: 60, Credits: 20},
}

// 走完阶梯之后每满这么多天再送一次,免得 60 天以后再无奔头——那正是最该留住
// 的一批人。
const (
	streakCycleDays    = 30
	streakCycleCredits = 10
)

// streakReward 返回这个连签天数该额外送几次,以及记进账本的来源标签。
func streakReward(streak int) (int, string) {
	for _, r := range StreakRewards {
		if streak == r.Days {
			return r.Credits, fmt.Sprintf("%s_%d", models.AIGrantStreak, r.Days)
		}
	}
	last := StreakRewards[len(StreakRewards)-1].Days
	if streak > last && (streak-last)%streakCycleDays == 0 {
		return streakCycleCredits, fmt.Sprintf("%s_%d", models.AIGrantStreak, streak)
	}
	return 0, ""
}

// monthEnd 是本月配给的过期时刻:下月一号零点。
//
// 月配给"不累积"就是这么实现的——不再需要一段"发现月份变了就重置"的代码,
// 上个月的那笔到点自己失效。而重置那种写法会连签到送的一起抹掉,那些本该
// 活满 60 天。
func monthEnd(t time.Time) time.Time {
	y, m, _ := t.UTC().Date()
	return time.Date(y, m+1, 1, 0, 0, 0, 0, time.UTC)
}

// grantTx 记一笔发放。expires 为 nil 表示永不过期。
//
// 批次与 users.ai_credits 这份汇总必须一起变,所以只接受事务。
func grantTx(tx *gorm.DB, userID uuid.UUID, amount int, reason string, expires *time.Time) error {
	if amount <= 0 {
		return nil
	}
	g := models.AICreditGrant{
		ID:        uuid.New(),
		UserID:    userID,
		Amount:    amount,
		Remaining: amount,
		Reason:    reason,
		ExpiresAt: expires,
	}
	if err := tx.Create(&g).Error; err != nil {
		return err
	}
	return tx.Model(&models.User{}).Where("id = ?", userID).
		UpdateColumn("ai_credits", gorm.Expr("ai_credits + ?", amount)).Error
}

// expireTx 把已过期批次的余额清零,并从汇总里减掉,返回清掉的次数。
//
// 惰性结算,没有定时任务:这个项目刻意不跑 cron,所以过期发生在"下一次有人
// 看余额或者花额度"的时候。代价是数据库里会短暂存在一些"已经过期但还没清"
// 的行——读余额的每条路径都先走这里,所以对外看不出来。
func expireTx(tx *gorm.DB, userID uuid.UUID, now time.Time) (int, error) {
	var expired []models.AICreditGrant
	if err := tx.Where("user_id = ? AND remaining > 0 AND expires_at IS NOT NULL AND expires_at <= ?",
		userID, now).Find(&expired).Error; err != nil {
		return 0, err
	}
	if len(expired) == 0 {
		return 0, nil
	}
	total := 0
	ids := make([]uuid.UUID, 0, len(expired))
	for _, g := range expired {
		total += g.Remaining
		ids = append(ids, g.ID)
	}
	if err := tx.Model(&models.AICreditGrant{}).Where("id IN ?", ids).
		UpdateColumn("remaining", 0).Error; err != nil {
		return 0, err
	}
	// GREATEST 兜底:汇总一旦因为别处的 bug 比批次总额小,这里再减就成负数,
	// 而负余额会让条件更新那道闸门永远开着。
	return total, tx.Model(&models.User{}).Where("id = ?", userID).
		UpdateColumn("ai_credits", gorm.Expr("GREATEST(ai_credits - ?, 0)", total)).Error
}

// consumeTx 按"先过期的先扣"从批次里扣掉 n 次。
//
// 顺序不是随意定的:后发的签到赠送往往比当月配给活得久,先扣快过期的那批,
// 用户手里剩下的才是最经花的。反过来扣会让人眼睁睁看着额度过期。
func consumeTx(tx *gorm.DB, userID uuid.UUID, n int, now time.Time) error {
	if n <= 0 {
		return nil
	}
	var live []models.AICreditGrant
	if err := tx.Where("user_id = ? AND remaining > 0 AND (expires_at IS NULL OR expires_at > ?)",
		userID, now).
		Order("expires_at ASC NULLS LAST, created_at ASC").
		Find(&live).Error; err != nil {
		return err
	}
	left := n
	for _, g := range live {
		if left <= 0 {
			break
		}
		take := g.Remaining
		if take > left {
			take = left
		}
		if err := tx.Model(&models.AICreditGrant{}).Where("id = ?", g.ID).
			UpdateColumn("remaining", gorm.Expr("remaining - ?", take)).Error; err != nil {
			return err
		}
		left -= take
	}
	if left > 0 {
		// 汇总说够、批次说不够。以批次为准并把汇总校准回去——反过来信汇总
		// 就等于凭空发次数。
		log.Printf("ai: 批次与汇总不一致 user=%s 缺 %d 次,已按批次校准", userID, left)
		if err := syncTotalTx(tx, userID, now); err != nil {
			return err
		}
		return ErrNoCredits
	}
	// 汇总跟着批次一起减。GREATEST 的理由同 expireTx:负余额会让扣减那道
	// 条件闸门永远开着。
	return tx.Model(&models.User{}).Where("id = ?", userID).
		UpdateColumn("ai_credits", gorm.Expr("GREATEST(ai_credits - ?, 0)", n)).Error
}

// syncTotalTx 用批次的实际余额校准 users.ai_credits。
func syncTotalTx(tx *gorm.DB, userID uuid.UUID, now time.Time) error {
	var sum int64
	if err := tx.Model(&models.AICreditGrant{}).
		Where("user_id = ? AND remaining > 0 AND (expires_at IS NULL OR expires_at > ?)", userID, now).
		Select("COALESCE(SUM(remaining), 0)").Scan(&sum).Error; err != nil {
		return err
	}
	return tx.Model(&models.User{}).Where("id = ?", userID).
		UpdateColumn("ai_credits", sum).Error
}

// LiveGrants 是用户当下还有效的发放明细,按最先过期的排在前面。
// 界面用它把"56 次"拆成看得懂的几笔。
func LiveGrants(db *gorm.DB, userID uuid.UUID) ([]models.AICreditGrant, error) {
	now := time.Now()
	var out []models.AICreditGrant
	err := db.Where("user_id = ? AND remaining > 0 AND (expires_at IS NULL OR expires_at > ?)",
		userID, now).
		Order("expires_at ASC NULLS LAST, created_at ASC").
		Find(&out).Error
	return out, err
}

// grantedLiveTx 是当前还有效的赠送额度,不含每月配给。
//
// 上限只该卡赠送:配给每月自动发下来,把它算进上限的话,越活跃的用户越会被
// 自己的配给挤掉签到奖励。
func grantedLiveTx(tx *gorm.DB, userID uuid.UUID, now time.Time) (int, error) {
	var sum int64
	err := tx.Model(&models.AICreditGrant{}).
		Where("user_id = ? AND remaining > 0 AND reason <> ? AND (expires_at IS NULL OR expires_at > ?)",
			userID, models.AIGrantMonthly, now).
		Select("COALESCE(SUM(remaining), 0)").Scan(&sum).Error
	return int(sum), err
}
