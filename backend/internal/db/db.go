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
	); err != nil {
		log.Fatalf("automigrate: %v", err)
	}
	seedDefaults(db)
	return db
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
