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

	// StorageDir backs the Local development storage driver. In production
	// every object lives in MinIO/S3 and this is only used for the temp
	// working files the upload pipeline writes while transcoding.
	StorageDir    string
	TempDir       string
	PublicBaseURL string
	CookieDomain  string
	CookieSecure  bool

	JWTSecret string
	// StorageMasterKey (base64, 32 bytes) encrypts user-supplied bucket
	// credentials at rest. Required once BYOS is enabled.
	StorageMasterKey string

	GoogleClientID     string
	GoogleClientSecret string
	GithubClientID     string
	GithubClientSecret string

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
		ListenAddr:       getenv("LISTEN_ADDR", "127.0.0.1:8080"),
		DatabaseURL:      mustGet("DATABASE_URL"),
		StorageDir:       getenv("STORAGE_DIR", "/opt/openimg/storage"),
		TempDir:          getenv("TEMP_DIR", "/opt/openimg/tmp"),
		PublicBaseURL:    getenv("PUBLIC_BASE_URL", "http://127.0.0.1:8080"),
		CookieDomain:     getenv("COOKIE_DOMAIN", ""),
		CookieSecure:     secure,
		JWTSecret:        mustGet("JWT_SECRET"),
		StorageMasterKey: os.Getenv("STORAGE_MASTER_KEY"),

		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GithubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GithubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),

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
