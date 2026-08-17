package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/gentpan/openimg/backend/internal/imageproc"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/scheduler"
	"github.com/gentpan/openimg/backend/internal/storage"
	"gorm.io/gorm"
)

// RegisterJobs wires the background handlers. Called from main after the
// server is constructed, because every handler needs DB + storage access.
func (s *Server) RegisterJobs() {
	if s.Queue == nil {
		return
	}
	s.Queue.Register(scheduler.JobTranscode, s.jobTranscode)
	s.Queue.Register(scheduler.JobBackup, s.jobBackup)
	s.Queue.Register(scheduler.JobPurge, s.jobPurge)
	s.Queue.Register(scheduler.JobAIPoll, s.jobAIPoll)
}

// RequeuePendingJobs re-enqueues work that was in flight when the process last
// stopped. The queue is in-memory, so without this a restart during a backup
// backlog would strand those images in `pending` forever.
func (s *Server) RequeuePendingJobs() {
	if s.Queue == nil {
		return
	}
	var pending []models.Image
	if err := s.DB.
		Where("backup_state IN ? AND deleted_at IS NULL",
			[]models.BackupState{models.BackupPending, models.BackupFailed}).
		Limit(5000).Find(&pending).Error; err != nil {
		log.Printf("scheduler: could not requeue pending backups: %v", err)
		return
	}
	for _, img := range pending {
		s.Queue.Submit(scheduler.JobBackup, img.ID)
	}

	// Soft-deleted rows whose objects were never purged (the process died
	// between the DELETE and the purge job).
	var orphaned []models.Image
	if err := s.DB.Unscoped().
		Where("deleted_at IS NOT NULL AND status = ?", models.ImageDeleted).
		Limit(5000).Find(&orphaned).Error; err == nil {
		for _, img := range orphaned {
			s.Queue.Submit(scheduler.JobPurge, img.ID)
		}
	}
	if n := len(pending) + len(orphaned); n > 0 {
		log.Printf("scheduler: requeued %d pending job(s)", n)
	}

	// AI 生成同理:队列在内存里,重启时正在轮询的那些会原地不动,而它们的
	// 额度已经扣了。这里只捡启动这一刻的存量,运行期间丢掉的由 StartAIReconciler
	// 的巡检兜住。
	s.reconcileAIGenerations(context.Background())
}

// jobTranscode generates an AVIF derivative alongside the primary. 转换语义
// 落地后上传路径不再入队(选 AVIF 时主图同步编码,原样模式转换不参与),
// 此处理器保留只为消化历史遗留任务,没有新的入队来源。
func (s *Server) jobTranscode(ctx context.Context, job scheduler.Job) error {
	var img models.Image
	if err := s.DB.First(&img, "id = ?", job.ImageID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // deleted before we got to it
		}
		return err
	}
	if img.DeletedAt != nil || img.HasVariant(models.VariantAVIF) {
		return nil
	}
	// Re-check the owner's preference: a job queued before they switched away
	// from AVIF must not resurrect it.
	var owner models.User
	if err := s.DB.Select("variant_format").First(&owner, "id = ?", img.UserID).Error; err == nil {
		if owner.VariantFormat != models.VariantAVIF {
			return nil
		}
	}

	backend, profile, err := s.Storage.ForStored(ctx, img.ProfileID)
	if err != nil {
		return err
	}
	_ = profile

	rc, err := backend.Get(ctx, img.ObjectKey)
	if err != nil {
		return fmt.Errorf("read original: %w", err)
	}
	raw, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return fmt.Errorf("read original body: %w", err)
	}

	out, err := imageproc.MakeAVIF(raw)
	if err != nil {
		// Animated or otherwise unsuitable sources are a normal outcome, not a
		// failure worth retrying three times.
		if strings.Contains(err.Error(), "animated") {
			return nil
		}
		return fmt.Errorf("avif encode: %w", err)
	}
	// Only keep AVIF when it actually wins. On small or already-efficient
	// images it often loses to WebP, and storing a bigger "optimised" copy
	// costs space for nothing.
	if out.Size() >= img.SizeStored {
		return nil
	}

	key := models.VariantKey(img.ObjectKey, models.VariantAVIF)
	if err := backend.Put(ctx, key, bytes.NewReader(out.Data), out.Size(), out.MIME); err != nil {
		return fmt.Errorf("put avif: %w", err)
	}

	// Every row sharing this object gains the variant — the object is shared,
	// so the metadata must be too. 按 object_key 而非 sha 圈定:同字节可能
	// 因转换设置不同而以两种格式各存一份,变体 key 是从各自 object_key 推
	// 导的,标到别的格式的行上会得到一个不存在的链接。
	variants := img.Variants
	if variants == "" {
		variants = models.VariantAVIF
	} else if !img.HasVariant(models.VariantAVIF) {
		variants += "," + models.VariantAVIF
	}
	return s.DB.Model(&models.Image{}).
		Where("profile_id = ? AND object_key = ? AND deleted_at IS NULL", img.ProfileID, img.ObjectKey).
		Updates(map[string]any{
			"variants":      variants,
			"size_stored":   gorm.Expr("size_stored + ?", out.Size()),
			"size_variants": gorm.Expr("size_variants + ?", out.Size()),
		}).Error
}

// jobBackup replicates an image's objects to the profile configured as its
// backup target.
func (s *Server) jobBackup(ctx context.Context, job scheduler.Job) error {
	var img models.Image
	if err := s.DB.First(&img, "id = ?", job.ImageID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if img.DeletedAt != nil {
		return nil
	}

	dst, dstProfile, err := s.Storage.BackupFor(ctx, img.ProfileID)
	if err != nil {
		return err
	}
	if dst == nil || dstProfile == nil {
		// Backup target removed since the job was queued.
		return s.DB.Model(&img).Update("backup_state", models.BackupNone).Error
	}
	src, _, err := s.Storage.ForStored(ctx, img.ProfileID)
	if err != nil {
		return err
	}

	keys := append([]string{img.ObjectKey}, variantKeys(img)...)
	for _, key := range keys {
		if _, exists, err := dst.Stat(ctx, key); err == nil && exists {
			continue // already replicated, e.g. by a deduplicated sibling
		}
		if err := copyObject(ctx, src, dst, key); err != nil {
			s.DB.Model(&img).Updates(map[string]any{
				"backup_state": models.BackupFailed,
				"backup_error": truncate(err.Error(), 240),
			})
			return err
		}
	}
	return s.DB.Model(&img).Updates(map[string]any{
		"backup_state": models.BackupDone,
		"backup_error": "",
	}).Error
}

// jobPurge deletes the stored objects of a soft-deleted image — but only once
// no live row still points at them. Content addressing means several users can
// share one object, so an unconditional delete here would silently break other
// people's images.
func (s *Server) jobPurge(ctx context.Context, job scheduler.Job) error {
	var img models.Image
	if err := s.DB.Unscoped().First(&img, "id = ?", job.ImageID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	// 按 object_key 数引用:克隆共享的是对象键;同 sha 不同格式的行各自
	// 持有各自的对象,不该互相挡住清理(按 sha 数会让先删的那份成为桶里
	// 永远清不掉的孤儿)。
	var refs int64
	if err := s.DB.Model(&models.Image{}).
		Where("profile_id = ? AND object_key = ? AND deleted_at IS NULL", img.ProfileID, img.ObjectKey).
		Count(&refs).Error; err != nil {
		return err
	}
	if refs > 0 {
		return nil // someone else still uses this object
	}

	backend, _, err := s.Storage.ForStored(ctx, img.ProfileID)
	if err != nil {
		if errors.Is(err, storage.ErrProfileNotFound) {
			return nil
		}
		return err
	}
	keys := append([]string{img.ObjectKey}, variantKeys(img)...)
	for _, key := range keys {
		if err := backend.Delete(ctx, key); err != nil {
			return fmt.Errorf("delete %s: %w", key, err)
		}
	}
	// Mirror the deletion into the backup bucket so it doesn't become an
	// unbounded archive of everything anyone ever deleted.
	if dst, dstProfile, err := s.Storage.BackupFor(ctx, img.ProfileID); err == nil && dst != nil && dstProfile != nil {
		for _, key := range keys {
			if err := dst.Delete(ctx, key); err != nil {
				log.Printf("purge: backup delete %s failed: %v", key, err)
			}
		}
	}
	return nil
}

func variantKeys(img models.Image) []string {
	names := img.VariantList()
	out := make([]string, 0, len(names))
	for _, v := range names {
		out = append(out, models.VariantKey(img.ObjectKey, v))
	}
	return out
}

// copyObject streams one object between backends. Buffered rather than piped
// because S3 PutObject wants a known length, and a missing Content-Length
// forces chunked encoding that some S3 implementations reject.
func copyObject(ctx context.Context, src, dst storage.Backend, key string) error {
	rc, err := src.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("read %s: %w", key, err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("buffer %s: %w", key, err)
	}
	ct := storage.ContentTypeFor(extOf(key))
	return dst.Put(ctx, key, bytes.NewReader(data), int64(len(data)), ct)
}

func extOf(key string) string {
	if i := strings.LastIndex(key, "."); i >= 0 {
		return key[i+1:]
	}
	return ""
}
