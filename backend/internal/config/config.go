package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	ListenAddr  string
	DatabaseURL string

	// APIMart 文生图。留空即不开这个功能——接口一律回"未开启",界面把入口
	// 藏起来,而不是让用户点一下才收到 401。
	APIMartKey  string
	APIMartBase string

	// StorageDir backs the Local development storage driver. In production
	// every object lives in MinIO/S3 and this is only used for the temp
	// working files the upload pipeline writes while transcoding.
	StorageDir    string
	TempDir       string
	PublicBaseURL string
	// ShortBaseURL 是短链挂的域名。留空即与 PublicBaseURL 同域。
	//
	// 单独一个配置而不是写死:短链的价值在于短,值得为它用一个更短的域名;
	// 但自建部署多半只有一个域名,不该被迫再买一个。留空回落到主站,行为与
	// 加这个配置之前一模一样。
	ShortBaseURL string
	FrontendDir  string
	// AppleAppID is "TEAMID.bundleid" for the Mac app. Set it to serve the
	// associated-domains document that lets the app use a passkey natively
	// instead of through a web sheet. Unset, the document is not served.
	AppleAppID string
	// MacUpdateManifest 指向 macOS 客户端更新清单的文件路径。留空即不提供
	// 自动更新,那条路由会返回明确的 404。
	MacUpdateManifest string
	CookieDomain      string
	CookieSecure      bool

	JWTSecret string
	// StorageMasterKey (base64, 32 bytes) encrypts user-supplied bucket
	// credentials at rest. Required once BYOS is enabled.
	StorageMasterKey string

	GoogleClientID     string
	GoogleClientSecret string
	GithubClientID     string
	GithubClientSecret string

	// pic.bi 关联。两套凭据,用途完全不同,不要合并:
	//
	//   - OAuth 那对(ClientID/Secret)是"让用户把两个账号连起来",走浏览器
	//     重定向,用户看得见同意页。
	//   - partner 那对(PartnerID/Secret)是服务端到服务端的 HMAC 共享密钥,
	//     用来查余额和扣费,用户永远看不到。
	//
	// 任何一组留空,对应的能力就整个关掉:没有 OAuth 就没人能关联,没有
	// partner 密钥就没人能被扣费(4K 档也随之关闭)。半配置状态下的表现是
	// 每次调用都被对端拒绝,而那看起来像"pic.bi 坏了"。
	PicbiBaseURL       string
	PicbiClientID      string
	PicbiClientSecret  string
	PicbiPartnerID     string
	PicbiPartnerSecret string

	// Email. EmailProvider selects the backend: "cloudflare" | "sendflare" |
	// "" (auto — whichever has credentials, Cloudflare first).
	EmailProvider       string
	EmailFrom           string
	EmailFromName       string
	CloudflareAccountID string
	CloudflareAPIToken  string
	SendflareAPIKey     string

	// Platform pool (MinIO). Seeded into a StorageProfile row on first boot;
	// afterwards the DB row is the source of truth and these are ignored.
	S3Endpoint      string
	S3Region        string
	S3Bucket        string
	S3AccessKey     string
	S3SecretKey     string
	S3PublicURLBase string
	S3ThumbURLBase  string
	S3KeyPrefix     string
	S3UsePathStyle  bool

	// Upload pipeline.
	MaxUploadSize   int64 // hard ceiling, above any group's MaxFileSize
	WorkerCount     int
	VipsConcurrency int

	RequireEmailVerified bool
}

func Load() Config {
	_ = godotenv.Load("/opt/openimg/config/.env")
	_ = godotenv.Load(".env")

	workers, _ := strconv.Atoi(getenv("WORKER_COUNT", "4"))
	vipsConc, _ := strconv.Atoi(getenv("VIPS_CONCURRENCY", "2"))
	maxUpload, _ := strconv.ParseInt(getenv("MAX_UPLOAD_SIZE", "52428800"), 10, 64) // 50 MiB
	secure, _ := strconv.ParseBool(getenv("COOKIE_SECURE", "false"))
	requireVerified, _ := strconv.ParseBool(getenv("REQUIRE_EMAIL_VERIFIED", "true"))

	cfg := Config{
		APIMartKey:    getenv("APIMART_API_KEY", ""),
		APIMartBase:   getenv("APIMART_BASE_URL", ""),
		ListenAddr:    getenv("LISTEN_ADDR", "127.0.0.1:8080"),
		DatabaseURL:   mustGet("DATABASE_URL"),
		StorageDir:    getenv("STORAGE_DIR", "/opt/openimg/storage"),
		TempDir:       getenv("TEMP_DIR", "/opt/openimg/tmp"),
		PublicBaseURL: getenv("PUBLIC_BASE_URL", "http://127.0.0.1:8080"),
		ShortBaseURL:  strings.TrimRight(strings.TrimSpace(os.Getenv("SHORT_BASE_URL")), "/"),
		// Optional. Set it to the frontend's dist/ and this process serves the
		// SPA too, which is what lets short links live at the domain root.
		FrontendDir:       os.Getenv("FRONTEND_DIR"),
		AppleAppID:        os.Getenv("APPLE_APP_ID"),
		MacUpdateManifest: os.Getenv("MAC_UPDATE_MANIFEST"),
		CookieDomain:      getenv("COOKIE_DOMAIN", ""),
		CookieSecure:      secure,
		JWTSecret:         mustGet("JWT_SECRET"),
		StorageMasterKey:  os.Getenv("STORAGE_MASTER_KEY"),

		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GithubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GithubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),

		PicbiBaseURL:       strings.TrimRight(strings.TrimSpace(os.Getenv("PICBI_BASE_URL")), "/"),
		PicbiClientID:      os.Getenv("PICBI_CLIENT_ID"),
		PicbiClientSecret:  os.Getenv("PICBI_CLIENT_SECRET"),
		PicbiPartnerID:     os.Getenv("PICBI_PARTNER_ID"),
		PicbiPartnerSecret: os.Getenv("PICBI_PARTNER_SECRET"),

		EmailProvider:       strings.ToLower(strings.TrimSpace(os.Getenv("EMAIL_PROVIDER"))),
		EmailFrom:           getenv("EMAIL_FROM", "noreply@openimg.io"),
		EmailFromName:       getenv("EMAIL_FROM_NAME", "Openimg"),
		CloudflareAccountID: os.Getenv("CLOUDFLARE_ACCOUNT_ID"),
		CloudflareAPIToken:  os.Getenv("CLOUDFLARE_API_TOKEN"),
		SendflareAPIKey:     os.Getenv("SENDFLARE_API_KEY"),

		S3Endpoint:      os.Getenv("S3_ENDPOINT"),
		S3Region:        getenv("S3_REGION", "auto"),
		S3Bucket:        os.Getenv("S3_BUCKET"),
		S3AccessKey:     os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:     os.Getenv("S3_SECRET_KEY"),
		S3PublicURLBase: os.Getenv("S3_PUBLIC_URL_BASE"),
		S3ThumbURLBase:  os.Getenv("S3_THUMB_URL_BASE"),
		S3KeyPrefix:     os.Getenv("S3_KEY_PREFIX"),
		S3UsePathStyle:  parseBool(getenv("S3_PATH_STYLE", "true")),

		MaxUploadSize:        maxUpload,
		WorkerCount:          workers,
		VipsConcurrency:      vipsConc,
		RequireEmailVerified: requireVerified,
	}

	for _, dir := range []string{cfg.StorageDir, cfg.TempDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("create dir %s: %v", dir, err)
		}
	}
	return cfg
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func parseBool(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}

func mustGet(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("required env var %s is empty", k)
	}
	return v
}
