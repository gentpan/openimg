package db

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	mib = int64(1) << 20
	gib = int64(1) << 30
)

// Open connects and, unless autoMigrate is false, syncs the schema. Turning it
// off skips the full-table scans GORM does per model on every restart — worth
// it in production once the schema is stable; keep it on while models churn.
func Open(dsn string, autoMigrate bool) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	// GORM's defaults are tuned for "just connect": no cap on open
	// connections, and idle ones are closed immediately so every burst
	// re-dials. Upload bursts and the scheduler workers all hit this pool,
	// so size it explicitly instead.
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("db pool: %v", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	if !autoMigrate {
		log.Printf("db: auto-migrate disabled (DB_AUTO_MIGRATE=false)")
		seedDefaults(db)
		return db
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.UserGroup{},
		&models.Session{},
		&models.EmailOTP{},
		&models.PasskeyCredential{},
		&models.StorageProfile{},
		&models.APIToken{},
		&models.Image{},
		&models.QuotaTransaction{},
		&models.CheckinRecord{},
		&models.Report{},
		&models.SiteSetting{},
		&models.Reaction{},
		&models.AIGeneration{},
		&models.AICreditGrant{},
	); err != nil {
		log.Fatalf("automigrate: %v", err)
	}
	seedDefaults(db)
	migrateAIGrants(db)
	return db
}

// migrateAIGrants 把批次账本上线之前的存量余额补成一笔发放。
//
// 不补的话老用户的 users.ai_credits 还在,但批次是空的,第一次生成就会撞上
// "汇总说够、批次说不够",被校准成 0——余额凭空蒸发。
//
// 过期时间给到月底,与从前的语义一致:那时余额本来就是每月重置的,存量这笔
// 本来也活不过这个月。
//
// 幂等靠"一条批次都没有"这个条件:跑过一次之后这些用户就有记录了,重启不会
// 重复补发。
func migrateAIGrants(db *gorm.DB) {
	var users []models.User
	if err := db.Where("ai_credits > 0 AND id NOT IN (SELECT user_id FROM ai_credit_grants)").
		Find(&users).Error; err != nil {
		log.Printf("ai 批次迁移: 查存量失败: %v", err)
		return
	}
	if len(users) == 0 {
		return
	}
	now := time.Now().UTC()
	exp := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	n := 0
	for _, u := range users {
		g := models.AICreditGrant{
			ID: uuid.New(), UserID: u.ID,
			Amount: u.AICredits, Remaining: u.AICredits,
			Reason: models.AIGrantMigrated, ExpiresAt: &exp,
		}
		if err := db.Create(&g).Error; err != nil {
			log.Printf("ai 批次迁移: user=%s 失败: %v", u.ID, err)
			continue
		}
		n++
	}
	log.Printf("ai 批次迁移: %d 个账号的存量余额已转成批次", n)
}

func seedDefaults(db *gorm.DB) {
	// Tiers. Allowances are deliberately modest — this is a donated pool, and
	// headroom is earned by showing up rather than by signing up.
	groups := []models.UserGroup{
		{
			Name: "admin", Description: "管理员（无限制）",
			MaxFileSize: 200 * mib, DailyUploadCount: 100000,
			AllowedFormats: "jpeg,png,webp,gif,avif,heic,bmp,tiff",
			MaxWidth:       30000, MaxHeight: 30000,
			SignupSpace: 100 * gib, CheckinMinSpace: 1 * mib, CheckinMaxSpace: 30 * mib,
			StreakBonusSpace: 0, StreakBonusDays: 7,
			ReferralSpace: 1 * gib, MaxTotalSpace: 0, // uncapped
			AllowBYOS: true, MaxProfiles: 20,
		},
		{
			Name: "trusted", Description: "受信任用户（长期活跃）",
			MaxFileSize: 50 * mib, DailyUploadCount: 500,
			AllowedFormats: "jpeg,png,webp,gif,avif,heic,tiff",
			MaxWidth:       16000, MaxHeight: 16000,
			SignupSpace: 5 * gib, CheckinMinSpace: 1 * mib, CheckinMaxSpace: 30 * mib,
			StreakBonusSpace: 200 * mib, StreakBonusDays: 7,
			ReferralSpace: 500 * mib, MaxTotalSpace: 0, // uncapped
			AllowBYOS: true, MaxProfiles: 10,
		},
		{
			Name: "free", Description: "注册免费用户",
			MaxFileSize: 20 * mib, DailyUploadCount: 100,
			// HEIC because that is what an iPhone shoots by default —
			// excluding it excludes uploading straight from a photo library.
			AllowedFormats: "jpeg,png,webp,gif,avif,heic",
			MaxWidth:       8000, MaxHeight: 8000,
			SignupSpace: 1 * gib, CheckinMinSpace: 1 * mib, CheckinMaxSpace: 30 * mib,
			StreakBonusSpace: 30 * mib, StreakBonusDays: 7,
			ReferralSpace: 200 * mib, MaxTotalSpace: 0, // uncapped
			AllowBYOS: true, MaxProfiles: 3,
		},
	}
	for _, g := range groups {
		var existing models.UserGroup
		if err := db.Where("name = ?", g.Name).First(&existing).Error; err == gorm.ErrRecordNotFound {
			g.ID = uuid.New()
			db.Create(&g)
			log.Printf("seeded group: %s", g.Name)
		}
	}

	// Bootstrap admin from env if specified.
	adminEmail := strings.ToLower(strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL")))
	adminPass := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	if adminEmail != "" {
		var u models.User
		if err := db.Where("email = ?", adminEmail).First(&u).Error; err == gorm.ErrRecordNotFound && adminPass != "" {
			hash, _ := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
			var adminGroup models.UserGroup
			db.Where("name = ?", "admin").First(&adminGroup)
			user := models.NewUser(adminEmail, "Admin", string(hash))
			user.Role = models.RoleAdmin
			user.EmailVerified = true
			user.GroupID = &adminGroup.ID
			db.Create(&user)
			// Grant through the ledger rather than setting QuotaBytes directly, so
			// the balance stays reconstructible from quota_transactions.
			if _, err := quota.SignupGrant(db, user.ID, &adminGroup); err != nil {
				log.Printf("bootstrap admin: signup grant failed: %v", err)
			}
			log.Printf("bootstrapped admin user: %s", adminEmail)
		} else if err == nil && u.Role != models.RoleAdmin {
			db.Model(&u).Update("role", models.RoleAdmin)
			log.Printf("promoted existing user to admin: %s", adminEmail)
		}
	}
}
