package main

import (
	"context"
	"log"
	"net/url"
	"os/signal"
	"syscall"
	"time"

	"github.com/gentpan/openimg/backend/internal/api"
	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/config"
	"github.com/gentpan/openimg/backend/internal/crypto"
	"github.com/gentpan/openimg/backend/internal/db"
	"github.com/gentpan/openimg/backend/internal/email"
	"github.com/gentpan/openimg/backend/internal/imageproc"
	"github.com/gentpan/openimg/backend/internal/passkey"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/gentpan/openimg/backend/internal/referral"
	"github.com/gentpan/openimg/backend/internal/scheduler"
	"github.com/gentpan/openimg/backend/internal/storage"
)

// pickEmailSender resolves the mail provider. An explicit EMAIL_PROVIDER wins;
// otherwise whichever one has credentials is used, preferring Cloudflare.
func pickEmailSender(cfg config.Config) email.Sender {
	// Providers get the full "Openimg <noreply@…>" form so recipients' inboxes
	// show the site name instead of the local part of the address.
	from := email.FormatFrom(cfg.EmailFromName, cfg.EmailFrom)
	cf := email.NewCloudflare(cfg.CloudflareAccountID, cfg.CloudflareAPIToken, from)
	sf := email.NewSendflare(cfg.SendflareAPIKey, from)

	switch cfg.EmailProvider {
	case "cloudflare":
		return cf
	case "sendflare":
		return sf
	case "none":
		return email.Disabled()
	}
	if cf.Configured() {
		return cf
	}
	if sf.Configured() {
		return sf
	}
	return email.Disabled()
}

// rpIDFor derives the WebAuthn Relying Party ID from the site's own origin.
//
// It used to be the literal "openimg.io", which meant WebAuthn was broken on
// every deployment that was not openimg.io: the RP ID has to be a registrable
// suffix of the page's origin, so a browser on img.example.com refuses to
// create or use a credential scoped to someone else's domain. Self-hosters got
// a passkey button that could never succeed.
//
// The port is dropped because RP IDs are domains, not origins — localhost:8080
// must register as "localhost", which the spec allows as a secure context.
func rpIDFor(baseURL string) string {
	if u, err := url.Parse(baseURL); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	log.Printf("passkey: cannot parse PUBLIC_BASE_URL %q, falling back to localhost", baseURL)
	return "localhost"
}

func main() {
	cfg := config.Load()
	gdb := db.Open(cfg.DatabaseURL)
	referral.BackfillCodes(gdb)
	if n, err := quota.BackfillMissingGrants(gdb); err != nil {
		log.Printf("quota: backfill failed: %v", err)
	} else if n > 0 {
		log.Printf("quota: backfilled ledger entries for %d user(s)", n)
	}

	cipher, err := crypto.New(cfg.StorageMasterKey)
	if err != nil {
		log.Fatalf("crypto: %v", err)
	}
	if !cipher.Enabled() {
		key, _ := crypto.GenerateKey()
		log.Printf("WARNING: STORAGE_MASTER_KEY is unset — bucket credentials cannot be stored. Generate one with:\n  STORAGE_MASTER_KEY=%s", key)
	}

	// Local filesystem stands in for the platform pool until MinIO is
	// configured, so a fresh checkout runs end to end.
	local := storage.NewLocal(cfg.StorageDir, cfg.PublicBaseURL)
	registry := storage.NewRegistry(gdb, cipher, local)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := registry.EnsurePlatformProfile(ctx, storage.S3Config{
		Endpoint:      cfg.S3Endpoint,
		Region:        cfg.S3Region,
		Bucket:        cfg.S3Bucket,
		AccessKey:     cfg.S3AccessKey,
		SecretKey:     cfg.S3SecretKey,
		PublicURLBase: cfg.S3PublicURLBase,
		ThumbURLBase:  cfg.S3ThumbURLBase,
		KeyPrefix:     cfg.S3KeyPrefix,
		UsePathStyle:  cfg.S3UsePathStyle,
	}); err != nil {
		log.Fatalf("storage: %v", err)
	}

	if err := imageproc.Startup(cfg.VipsConcurrency); err != nil {
		log.Fatalf("imageproc: libvips init failed: %v", err)
	}
	defer imageproc.Shutdown()

	authSvc := auth.New(gdb, cfg.JWTSecret, cfg.CookieSecure, cfg.CookieDomain)
	queue := scheduler.New(gdb)

	srv := api.New(gdb, queue, authSvc, registry, cipher)
	srv.StorageDir = cfg.StorageDir
	srv.PublicBaseURL = cfg.PublicBaseURL
	srv.FrontendDir = cfg.FrontendDir
	srv.ReactionSalt = cfg.JWTSecret
	// Existing rows predate short links; give them one.
	go srv.BackfillShortCodes()
	srv.TempDir = cfg.TempDir
	srv.MaxUploadSize = cfg.MaxUploadSize
	srv.RequireEmailVerified = cfg.RequireEmailVerified
	srv.CookieDomain = cfg.CookieDomain
	srv.CookieSecure = cfg.CookieSecure
	srv.OAuth = api.OAuthConfig{
		GoogleClientID:     cfg.GoogleClientID,
		GoogleClientSecret: cfg.GoogleClientSecret,
		GithubClientID:     cfg.GithubClientID,
		GithubClientSecret: cfg.GithubClientSecret,
		BaseURL:            cfg.PublicBaseURL,
	}
	srv.Email = pickEmailSender(cfg)
	log.Printf("email: provider=%s from=%s configured=%v",
		srv.Email.Name(), cfg.EmailFrom, srv.Email.Configured())
	if pk, err := passkey.New(gdb, rpIDFor(cfg.PublicBaseURL), "OpenIMG", cfg.PublicBaseURL); err == nil {
		srv.Passkey = pk
	} else {
		log.Printf("passkey init failed: %v", err)
	}

	srv.RegisterJobs()
	queue.Start(ctx, cfg.WorkerCount)
	srv.RequeuePendingJobs()
	r := srv.Router()

	log.Printf("listening on %s (workers=%d max-upload=%dMiB)",
		cfg.ListenAddr, cfg.WorkerCount, cfg.MaxUploadSize>>20)
	go func() {
		if err := r.Run(cfg.ListenAddr); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	time.Sleep(500 * time.Millisecond)
}
