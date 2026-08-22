// Command reprocess repairs images that were stored before two fixes landed:
// the PNG export filter (every PNG was written 30-80% larger than it arrived)
// and the grid thumbnail width (200px, which the gallery upscales past 2x).
//
// It re-encodes PNG primaries and adds the 600px tier, then corrects the size
// columns and refunds the quota difference.
//
// Deliberately narrow, because the alternative to a careful pass here is
// silently damaging images that are already correct:
//
//   - Only PNG primaries are re-encoded. PNG decode/encode is bit-exact, so
//     the pixels survive. Re-encoding a JPEG or WebP would degrade it every
//     time this runs, to save bytes that were never the problem.
//   - A new primary is written only when it is strictly smaller AND the
//     dimensions still match.
//   - The old w200 object stays. It costs ~6.5 KB and someone may have
//     embedded that URL; the listing prefers w600 wherever both exist.
//
// Objects are written before the row is updated. Getting that backwards would
// leave the database claiming sizes that do not exist in the bucket; this way
// the worst case is a correct-but-larger object still described by the old row,
// which the next run fixes.
//
//	go run ./cmd/reprocess            # 演练，不写任何东西
//	go run ./cmd/reprocess -apply     # 实际执行
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/gentpan/openimg/backend/internal/config"
	"github.com/gentpan/openimg/backend/internal/crypto"
	"github.com/gentpan/openimg/backend/internal/db"
	"github.com/gentpan/openimg/backend/internal/imageproc"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/gentpan/openimg/backend/internal/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func main() {
	apply := flag.Bool("apply", false, "实际写入；默认只演练并打印将要发生的变化")
	limit := flag.Int("limit", 0, "只处理前 N 张，0 表示全部")
	only := flag.String("id", "", "只处理指定的图片 ID")
	flag.Parse()

	cfg := config.Load()
	gdb := db.Open(cfg.DatabaseURL, true)

	cipher, err := crypto.New(cfg.StorageMasterKey)
	if err != nil {
		log.Fatalf("crypto: %v", err)
	}
	local := storage.NewLocal(cfg.StorageDir, cfg.PublicBaseURL)
	registry := storage.NewRegistry(gdb, cipher, local)

	ctx := context.Background()
	if err := registry.EnsurePlatformProfile(ctx, storage.S3Config{
		Endpoint: cfg.S3Endpoint, Region: cfg.S3Region, Bucket: cfg.S3Bucket,
		AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey,
		PublicURLBase: cfg.S3PublicURLBase, ThumbURLBase: cfg.S3ThumbURLBase,
		KeyPrefix: cfg.S3KeyPrefix, UsePathStyle: cfg.S3UsePathStyle,
	}); err != nil {
		log.Fatalf("storage: %v", err)
	}
	if err := imageproc.Startup(cfg.VipsConcurrency); err != nil {
		log.Fatalf("imageproc: %v", err)
	}
	defer imageproc.Shutdown()

	q := gdb.Where("deleted_at IS NULL AND status = ?", models.ImageActive).Order("created_at")
	if *only != "" {
		id, err := uuid.Parse(*only)
		if err != nil {
			log.Fatalf("非法的图片 ID：%v", err)
		}
		q = q.Where("id = ?", id)
	}
	if *limit > 0 {
		q = q.Limit(*limit)
	}
	var images []models.Image
	if err := q.Find(&images).Error; err != nil {
		log.Fatalf("查询失败：%v", err)
	}

	if !*apply {
		fmt.Println("== 演练模式，不会写入任何东西；确认无误后加 -apply ==")
	}
	fmt.Printf("待处理 %d 张\n\n", len(images))

	var stats struct{ touched, skipped, failed int; before, after int64 }
	for i := range images {
		img := &images[i]
		// Read before repair(), which updates SizeStored in place — including
		// in dry-run, so the totals show what would happen.
		before := img.SizeStored
		res, err := repair(ctx, gdb, registry, img, *apply)
		stats.before += before
		stats.after += img.SizeStored
		switch {
		case err != nil:
			stats.failed++
			fmt.Printf("  ✗ %-44s %v\n", trim(img.OrigName, 44), err)
		case res == "":
			stats.skipped++
		default:
			stats.touched++
			fmt.Printf("  ✓ %-44s %s\n", trim(img.OrigName, 44), res)
		}
	}

	fmt.Printf("\n处理 %d 张，跳过 %d 张，失败 %d 张\n", stats.touched, stats.skipped, stats.failed)
	fmt.Printf("存储占用 %s → %s（%+s）\n",
		humanBytes(stats.before), humanBytes(stats.after), humanBytes(stats.after-stats.before))
	if !*apply {
		fmt.Println("\n以上为演练结果，未写入。加 -apply 实际执行。")
	}
	if stats.failed > 0 {
		os.Exit(1)
	}
}

// repair returns a human-readable summary of what changed, or "" when the
// image already had nothing to fix.
func repair(ctx context.Context, gdb *gorm.DB, reg *storage.Registry, img *models.Image, apply bool) (string, error) {
	backend, _, err := reg.ForID(ctx, img.ProfileID)
	if err != nil {
		return "", fmt.Errorf("取存储后端：%w", err)
	}

	rc, err := backend.Get(ctx, img.ObjectKey)
	if err != nil {
		return "", fmt.Errorf("下载主图：%w", err)
	}
	src, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return "", fmt.Errorf("读取主图：%w", err)
	}

	var notes []string
	newPrimary := int64(len(src))
	var primaryData []byte

	// PNG only — see the package comment.
	if strings.EqualFold(img.Ext, "png") {
		out, perr := imageproc.Process(src, imageproc.Options{SkipVariants: true})
		if perr != nil {
			return "", fmt.Errorf("重编码：%w", perr)
		}
		switch {
		case out.Primary.Width != img.Width || out.Primary.Height != img.Height:
			// Never silently swap in an image of a different size.
			return "", fmt.Errorf("重编码后尺寸变了：%dx%d → %dx%d，已跳过",
				img.Width, img.Height, out.Primary.Width, out.Primary.Height)
		case out.Primary.Size() < int64(len(src)):
			primaryData = out.Primary.Data
			newPrimary = out.Primary.Size()
			notes = append(notes, fmt.Sprintf("主图 %s→%s (%.0f%%)",
				humanBytes(int64(len(src))), humanBytes(newPrimary),
				100*float64(newPrimary)/float64(len(src))-100))
		}
	}

	// 600px tier, if it isn't there yet.
	var thumbData []byte
	var thumbMIME string
	newThumbs := img.SizeThumbs
	if !img.HasVariant(models.VariantW600) {
		th, terr := imageproc.MakeThumbnail(src, imageproc.GridThumbWidth)
		switch {
		case errors.Is(terr, imageproc.ErrNotWorthIt):
			// Already at or below 600px; the full image is the thumbnail.
		case terr != nil:
			return "", fmt.Errorf("生成 w600：%w", terr)
		default:
			thumbData, thumbMIME = th.Data, th.MIME
			newThumbs += th.Size()
			notes = append(notes, "+w600 "+humanBytes(th.Size()))
		}
	}

	if len(notes) == 0 {
		return "", nil
	}
	if !apply {
		img.SizeStored = newPrimary + img.SizeVariants + newThumbs
		return strings.Join(notes, "  "), nil
	}

	// Objects first: a row that under-reports what is in the bucket is
	// recoverable, a row that over-reports is not.
	if primaryData != nil {
		if err := backend.Put(ctx, img.ObjectKey, bytes.NewReader(primaryData),
			int64(len(primaryData)), img.MIME); err != nil {
			return "", fmt.Errorf("上传主图：%w", err)
		}
	}
	if thumbData != nil {
		key := models.VariantKey(img.ObjectKey, models.VariantW600)
		if err := backend.Put(ctx, key, bytes.NewReader(thumbData),
			int64(len(thumbData)), thumbMIME); err != nil {
			return "", fmt.Errorf("上传 w600：%w", err)
		}
	}

	oldStored := img.SizeStored
	variants := img.VariantList()
	if thumbData != nil {
		variants = append(variants, models.VariantW600)
	}
	newStored := newPrimary + img.SizeVariants + newThumbs

	if err := gdb.Model(img).Updates(map[string]any{
		"size_primary": newPrimary,
		"size_thumbs":  newThumbs,
		"size_stored":  newStored,
		"variants":     strings.Join(variants, ","),
	}).Error; err != nil {
		return "", fmt.Errorf("更新记录：%w", err)
	}
	img.SizeStored = newStored

	// Quota moves in whichever direction the bytes did.
	if d := newStored - oldStored; d != 0 {
		var qerr error
		if d < 0 {
			qerr = quota.Release(gdb, img.UserID, -d, "重新处理图片 "+img.OrigName, &img.ID)
		} else {
			qerr = quota.Consume(gdb, img.UserID, d, "重新处理图片 "+img.OrigName, &img.ID)
		}
		if qerr != nil {
			// The bytes and the row are already consistent; only the ledger
			// lags, and quota.Reconcile can repair that separately.
			notes = append(notes, "配额调整失败："+qerr.Error())
		}
	}
	return strings.Join(notes, "  "), nil
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func humanBytes(n int64) string {
	neg := ""
	if n < 0 {
		neg, n = "-", -n
	}
	const u = 1024
	if n < u {
		return fmt.Sprintf("%s%d B", neg, n)
	}
	div, exp := int64(u), 0
	for m := n / u; m >= u; m /= u {
		div *= u
		exp++
	}
	return fmt.Sprintf("%s%.1f %cB", neg, float64(n)/float64(div), "KMGT"[exp])
}
