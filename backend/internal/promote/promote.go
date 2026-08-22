// Package promote 管两件事:免费用户自动升到受信任组,以及长期活跃用户每半年
// 一次的扩容。
//
// 两条规则共用一个判断底座——"这个账号值不值得给更多资源"——但答的不是同一个
// 问题:升级看的是风险(身份可追溯、有沉淀、没劣迹),扩容看的是需求(真的在
// 持续用、真的快满了)。混在一起写会长成一坨谁也不敢改的条件。
package promote

import (
	"fmt"
	"log"
	"time"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const gib = 1 << 30

// 升级门槛。
const (
	// MinAccountAge 注册满多久才考虑。
	//
	// 14 天而不是 30:门槛的作用是滤掉开完就用的一次性账号,两周足够;而站点
	// 本身还很年轻,30 天会让这条规则在很长一段时间里等于不存在——一条从不
	// 触发的规则和没有规则是一回事。
	MinAccountAge = 14 * 24 * time.Hour

	// MinUploadDays 至少有多少个"不同的日子"上传过。
	//
	// 这一条是整套规则里最要紧的。看总张数分不出"持续在用"和"搬完就走":
	// 库里真实存在过一个账号,171 张图全挤在 3 天里,之后半个月没再传过,而
	// 按张数算它比谁都活跃。日子分散不了是刷不出来的。
	MinUploadDays = 3
)

// 需求门槛,满足其一即可。
const (
	NeedUsageRatio  = 0.60
	NeedTotalImages = 50
)

// 长期活跃扩容。
const (
	// LoyaltyPeriod 两次扩容之间至少隔多久。
	LoyaltyPeriod = 180 * 24 * time.Hour

	// LoyaltyMinActiveMonths 这半年里至少几个月有上传。
	//
	// 4 个月而不是 6:偶尔空一个月是正常人的样子,要求全勤等于只奖励机器。
	LoyaltyMinActiveMonths = 4

	// LoyaltyUsageRatio 快满了才给。空间是真花钱的东西,给用不着的人翻倍
	// 只是把账面负债做大。
	LoyaltyUsageRatio = 0.70

	// LoyaltyCap 扩容的天花板,防止翻倍复利跑飞:10 → 20 → 40 就停。
	LoyaltyCap = 40 * gib
)

// Result 说明这次检查做了什么,供调用方写日志或提示用户。
type Result struct {
	Promoted     bool
	PromotedTo   string
	GrantedBytes int64
	Loyalty      bool
}

func (r Result) Any() bool { return r.Promoted || r.Loyalty }

// Check 是惰性入口:够格就升级/扩容,不够格就什么都不做。
//
// g 传调用方手里那份就行(上传、签到路径上本来就查过);传零值会自己去查。
// 这么做是因为它挂在上传后面,而上传是最高频的那条路——能省的查询都得省。
//
// 没有定时任务(这个项目刻意不跑 cron),所以挂在用户自己会触发的动作上——
// 上传完、签到时。代价是"够格了但还没来站上"的账号要等到下次露面才升,而
// 那正好也是升级唯一有意义的时刻。
//
// 任何一步出错都只记日志不往上抛:这是一件顺带做的好事,不该让上传或签到
// 因为它失败。
func Check(db *gorm.DB, u *models.User, g models.UserGroup) Result {
	var out Result
	if u == nil {
		return out
	}
	if g.Name == "" {
		var err error
		if g, err = groupOf(db, u.GroupID); err != nil {
			log.Printf("promote: 读用户组失败 user=%s: %v", u.ID, err)
			return out
		}
	}

	if g.Name == "free" {
		ok, err := eligibleForTrust(db, u)
		if err != nil {
			log.Printf("promote: 升级判定失败 user=%s: %v", u.ID, err)
		} else if ok {
			if n, err := promoteToTrusted(db, u, g); err != nil {
				log.Printf("promote: 升级失败 user=%s: %v", u.ID, err)
			} else {
				out.Promoted, out.PromotedTo, out.GrantedBytes = true, "trusted", n
			}
		}
	}

	// 扩容与升级同一次调用里都可能发生:刚升上去的人如果本来就满足条件,
	// 没理由让他再等半年。
	if n, err := grantLoyalty(db, u); err != nil {
		log.Printf("promote: 扩容判定失败 user=%s: %v", u.ID, err)
	} else if n > 0 {
		out.Loyalty = true
		out.GrantedBytes += n
	}
	return out
}

// eligibleForTrust 是升级的四道必答题加一道选答题。
func eligibleForTrust(db *gorm.DB, u *models.User) (bool, error) {
	if !u.EmailVerified {
		return false, nil
	}
	if time.Since(u.CreatedAt) < MinAccountAge {
		return false, nil
	}
	clean, err := noOffences(db, u.ID)
	if err != nil || !clean {
		return false, err
	}

	days, total, err := uploadShape(db, u.ID)
	if err != nil {
		return false, err
	}
	if days < MinUploadDays {
		return false, nil
	}

	// 需求:要么快满了,要么确实传了不少。两者都没有的话,升上去也用不着。
	if usageRatio(u) >= NeedUsageRatio || total >= NeedTotalImages {
		return true, nil
	}
	return false, nil
}

// noOffences:没有被封的图,也没有悬而未决的举报。
//
// resolved 的举报不算数——那一栏只说明"处理过了",不区分成立还是驳回,拿它
// 当污点会把被恶意举报过的人永久挡在门外。被封的图才是明确的处罚结果。
func noOffences(db *gorm.DB, userID uuid.UUID) (bool, error) {
	var blocked int64
	if err := db.Model(&models.Image{}).
		Where("user_id = ? AND status = ?", userID, models.ImageBlocked).
		Count(&blocked).Error; err != nil {
		return false, err
	}
	if blocked > 0 {
		return false, nil
	}
	var open int64
	if err := db.Model(&models.Report{}).
		Joins("JOIN images ON images.id = reports.image_id").
		Where("images.user_id = ? AND reports.status = ?", userID, models.ReportOpen).
		Count(&open).Error; err != nil {
		return false, err
	}
	return open == 0, nil
}

// uploadShape 一次问出"传过多少天"和"一共多少张"。
func uploadShape(db *gorm.DB, userID uuid.UUID) (days, total int, err error) {
	var row struct {
		Days  int
		Total int
	}
	err = db.Model(&models.Image{}).
		Select("COUNT(DISTINCT created_at::date) AS days, COUNT(*) AS total").
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Scan(&row).Error
	return row.Days, row.Total, err
}

func usageRatio(u *models.User) float64 {
	if u.QuotaBytes <= 0 {
		return 0
	}
	return float64(u.UsedBytes) / float64(u.QuotaBytes)
}

// groupOf 与 api.groupFor 同一套兜底:GroupID 为空的账号按 free 算,而不是
// 直接失败——历史数据里确实有没分组的行,把它们卡在门外没有道理。
func groupOf(db *gorm.DB, id *uuid.UUID) (models.UserGroup, error) {
	var g models.UserGroup
	if id != nil {
		if err := db.First(&g, "id = ?", *id).Error; err == nil {
			return g, nil
		}
	}
	err := db.Where("name = ?", "free").First(&g).Error
	return g, err
}

// promoteToTrusted 换组,并把两组注册额度的差额补给用户。
//
// 补差额而不是把 QuotaBytes 设成 10G:那样会抹掉签到、邀请一点点攒起来的
// 部分,等于因为升级反而变少。
func promoteToTrusted(db *gorm.DB, u *models.User, from models.UserGroup) (int64, error) {
	var to models.UserGroup
	if err := db.First(&to, "name = ?", "trusted").Error; err != nil {
		return 0, err
	}
	diff := to.SignupSpace - from.SignupSpace
	if diff < 0 {
		diff = 0
	}
	if err := db.Model(&models.User{}).Where("id = ?", u.ID).
		UpdateColumn("group_id", to.ID).Error; err != nil {
		return 0, err
	}
	u.GroupID = &to.ID

	granted, err := quota.GrantCapacity(db, u.ID, diff, models.QuotaPromote,
		"升级为受信任用户", to.MaxTotalSpace)
	if err != nil {
		// 组已经换了,额度没补上。不回滚:换组本身就是用户该得的,而额度
		// 下次检查还会补——GrantCapacity 是按当前 QuotaBytes 算的。
		return 0, err
	}
	u.QuotaBytes += granted
	log.Printf("promote: user=%s 升级为 trusted,补 %d 字节", u.ID, granted)
	return granted, nil
}

// grantLoyalty 是每半年一次的活跃扩容:把现有额度再翻一份。
func grantLoyalty(db *gorm.DB, u *models.User) (int64, error) {
	if u.QuotaBytes >= LoyaltyCap {
		return 0, nil
	}
	if usageRatio(u) < LoyaltyUsageRatio {
		return 0, nil
	}

	// 距上次扩容够不够半年。从没扩过就拿注册时间当起点——刚注册的人不该
	// 第一天就领到"长期活跃"奖励。
	since := u.CreatedAt
	var last models.QuotaTransaction
	err := db.Where("user_id = ? AND type = ?", u.ID, models.QuotaLoyalty).
		Order("created_at DESC").First(&last).Error
	if err == nil {
		since = last.CreatedAt
	} else if err != gorm.ErrRecordNotFound {
		return 0, err
	}
	if time.Since(since) < LoyaltyPeriod {
		return 0, nil
	}

	months, err := activeMonths(db, u.ID, since)
	if err != nil {
		return 0, err
	}
	if months < LoyaltyMinActiveMonths {
		return 0, nil
	}

	granted, err := quota.GrantCapacity(db, u.ID, u.QuotaBytes, models.QuotaLoyalty,
		fmt.Sprintf("长期活跃扩容（半年内 %d 个月有上传）", months), LoyaltyCap)
	if err == quota.ErrCapped {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	u.QuotaBytes += granted
	log.Printf("promote: user=%s 长期活跃扩容 %d 字节", u.ID, granted)
	return granted, nil
}

// activeMonths 数 since 之后有多少个不同的月份传过图。
func activeMonths(db *gorm.DB, userID uuid.UUID, since time.Time) (int, error) {
	var n int64
	err := db.Model(&models.Image{}).
		Select("COUNT(DISTINCT date_trunc('month', created_at))").
		Where("user_id = ? AND created_at >= ? AND deleted_at IS NULL", userID, since).
		Scan(&n).Error
	return int(n), err
}
