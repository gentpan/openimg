package models

import (
	"time"

	"github.com/google/uuid"
)

// 发放来源。写进 Reason,供用户侧明细和事后对账读。
const (
	AIGrantMonthly  = "monthly"  // 每月配给
	AIGrantCheckin  = "checkin"  // 每日签到
	AIGrantStreak   = "streak"   // 连签里程碑,后面接天数:streak_10
	AIGrantRefund   = "refund"   // 生成失败退回
	AIGrantMigrated = "migrated" // 迁移到批次账本之前的存量余额
)

// AICreditGrant 是一笔 AI 生成次数的发放。
//
// 余额从"一个整数"改成"一叠批次",是因为额度开始有了有效期:签到送的 60 天
// 过期,每月配给月底清零。单个整数记不住"这几次是哪天送的",也就无法只让该
// 过期的那部分过期。
//
// 顺带补上了一个一直缺的东西——发放痕迹。在这之前签到送的次数只体现在
// users.ai_credits 上,送了几次、哪天送的,事后完全查不到。
//
// users.ai_credits 保留为这张表的冗余汇总:那个字段上挂着扣减的并发闸门
// (条件更新 ai_credits >= n),换成聚合查询就得另造一把锁。两者必须在同一
// 个事务里一起变。
type AICreditGrant struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID uuid.UUID `gorm:"type:uuid;not null;index:idx_ai_grants_user_exp,priority:1" json:"user_id"`

	Amount    int `gorm:"not null" json:"amount"`
	Remaining int `gorm:"not null" json:"remaining"`

	Reason string `gorm:"size:32;not null" json:"reason"`

	// ExpiresAt 为 NULL 表示永不过期。目前没有这种发放,但管理员手工补偿会
	// 需要。用指针而不是零值时间:SQL 里 IS NULL / NULLS LAST 是现成的,拿
	// 0001-01-01 去比大小则要在每条查询里重复一遍那个魔法日期。
	ExpiresAt *time.Time `gorm:"index:idx_ai_grants_user_exp,priority:2" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 显式钉死。GORM 的命名策略会把 AI 开头的驼峰切坏——UserGroup 上
// 的 AIDaily 就被切成了 a_idaily(它把 ID 当作固有缩写),那个错列名已经在库
// 里了,改要迁移。这张表不再犯。
func (AICreditGrant) TableName() string { return "ai_credit_grants" }

// Live 判断这笔发放此刻是否还算数。
func (g AICreditGrant) Live(now time.Time) bool {
	return g.Remaining > 0 && (g.ExpiresAt == nil || g.ExpiresAt.After(now))
}
