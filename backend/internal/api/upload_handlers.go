package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/i18n"
	"github.com/gentpan/openimg/backend/internal/imageproc"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/gentpan/openimg/backend/internal/scheduler"
	"github.com/gentpan/openimg/backend/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// uploadTimeout bounds the storage writes for one image. Transcoding happens
// before this clock starts; this is purely the network leg.
const uploadTimeout = 2 * time.Minute

// POST /api/upload — the one endpoint that matters.
//
// Ordering is deliberate: every cheap rejection happens before we spend CPU on
// decoding. Tier checks, then size, then hash, then dedup, and only then the
// transcode. An abusive client should cost us a database round trip, not a
// libvips invocation.
func (s *Server) handleUpload(c *gin.Context) {
	u := auth.MustUser(c)
	g := s.groupFor(u)
	maxSize, ok := s.uploadPreflight(c, u, &g)
	if !ok {
		return
	}

	// Cap the request body before reading a byte of it.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)

	fh, err := c.FormFile("file")
	if err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": i18n.T(c, "upload.too_large", float64(maxSize)/(1<<20)),
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "upload.missing_field")})
		return
	}
	if fh.Size > maxSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": i18n.T(c, "upload.too_large", float64(maxSize)/(1<<20)),
		})
		return
	}

	raw, sum, err := readAndHash(fh)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "upload.read_failed", err.Error())})
		return
	}
	s.finishUpload(c, u, &g, raw, sum, filepath.Base(fh.Filename))
}

// uploadPreflight 是两条上传路径(表单、按网址取)共用的那半:账号状态、每日
// 张数、单文件上限。返回 ok=false 时响应已经写出去了。
//
// 抽出来不是为了少写几行,是为了**不会只改一处**:这几道闸任何一道在某一条路
// 径上漏掉,那条路径就成了绕开限额的后门,而这种漏洞不会有任何症状。
func (s *Server) uploadPreflight(c *gin.Context, u *models.User, g *models.UserGroup) (int64, bool) {
	if s.RequireEmailVerified && !u.EmailVerified && !u.IsAdmin() {
		c.JSON(http.StatusForbidden, gin.H{
			"error": i18n.T(c, "upload.email_unverified"), "code": "email_unverified",
		})
		return 0, false
	}
	if u.Status != models.UserActive {
		c.JSON(http.StatusForbidden, gin.H{"error": i18n.T(c, "upload.suspended")})
		return 0, false
	}

	// Daily upload count, per tier.
	dayStart := time.Now().UTC().Truncate(24 * time.Hour)
	var todayCount int64
	s.DB.Model(&models.Image{}).
		Where("user_id = ? AND created_at >= ?", u.ID, dayStart).
		Count(&todayCount)
	if g.DailyUploadCount > 0 && todayCount >= int64(g.DailyUploadCount) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": i18n.T(c, "upload.daily_limit"),
			"used":  todayCount, "limit": g.DailyUploadCount,
		})
		return 0, false
	}

	// The tier limit is the real bound; the server-wide one is a backstop
	// against a misconfigured group.
	maxSize := g.MaxFileSize
	if maxSize <= 0 || (s.MaxUploadSize > 0 && maxSize > s.MaxUploadSize) {
		maxSize = s.MaxUploadSize
	}
	return maxSize, true
}

// finishUpload 是两条路径共用的后半:认格式、落存储、回结果。
func (s *Server) finishUpload(c *gin.Context, u *models.User, g *models.UserGroup,
	raw []byte, sum, origName string) {
	// Sniff the real format from the bytes. The filename extension and the
	// client's Content-Type are both attacker-controlled and neither is
	// consulted for anything but the display name.
	ext, err := imageproc.Detect(raw)
	if err != nil {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": err.Error()})
		return
	}
	if !g.AllowsFormat(ext) {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{
			"error":   i18n.T(c, "upload.format_denied", ext),
			"allowed": g.FormatList(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), uploadTimeout)
	defer cancel()

	backend, profile, err := s.Storage.Resolve(ctx, u)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": i18n.T(c, "storage.unavailable", err.Error())})
		return
	}
	onPlatform := profile == nil || profile.IsPlatform()

	img, dup, err := s.storeUpload(ctx, storeParams{
		User:       u,
		Group:      g,
		Backend:    backend,
		Profile:    profile,
		OnPlatform: onPlatform,
		Raw:        raw,
		SHA:        sum,
		Ext:        ext,
		OrigName:   origName,
		IP:         c.ClientIP(),
	})
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, quota.ErrInsufficient):
			status = http.StatusInsufficientStorage
		case errors.As(err, new(*imageproc.ErrTooLarge)), errors.As(err, new(*imageproc.ErrUnsupported)):
			status = http.StatusUnsupportedMediaType
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	out := s.decorate([]models.Image{*img})
	c.JSON(http.StatusCreated, gin.H{"image": out[0], "deduplicated": dup})
}

type storeParams struct {
	User       *models.User
	Group      *models.UserGroup
	Backend    storage.Backend
	Profile    *models.StorageProfile
	OnPlatform bool
	Raw        []byte
	SHA        string
	Ext        string
	OrigName   string
	IP         string
}

// storeUpload does the transcode-and-persist half of an upload. Split from the
// handler so the API-token endpoint can share it verbatim.
//
// Returns (image, deduplicated, error).
func (s *Server) storeUpload(ctx context.Context, p storeParams) (*models.Image, bool, error) {
	profileID := uuid.Nil
	if p.Profile != nil {
		profileID = p.Profile.ID
	}

	// Dedup: the same bytes in the same bucket are stored once. We look for
	// surviving rows with this hash — including other users', since the
	// object is shared — and clone metadata instead of re-encoding.
	//
	// 但只克隆处理签名一致的旧行:同样的字节在不同设置(原样/优化、转换
	// 目标、限宽)下产物完全不同,克隆到别的设置的产物会让"改了设置再传
	// 同一张图"看起来没生效,甚至拿到语义不符的字节(原样用户克隆到被重
	// 编码的、优化用户克隆到 EXIF 俱在的)。签名一致意味着字节和设置都
	// 相同,产物必然逐位可复用。同字节因此可能按设置各存一份——设置生效
	// 优先于去重省空间;引用计数按 object_key 而非 sha 统计,互不干扰。
	// 存量旧行签名为空,自然判不相容,首次重传按新设置重新处理后收敛。
	sig := procSig(p.User)
	var twin models.Image
	dupErr := s.DB.Where("profile_id = ? AND sha256 = ? AND proc_sig = ? AND deleted_at IS NULL",
		profileID, p.SHA, sig).
		Order("created_at ASC").First(&twin).Error
	if dupErr == nil {
		img, err := s.cloneImage(ctx, twin, p)
		return img, true, err
	}
	if !errors.Is(dupErr, gorm.ErrRecordNotFound) {
		return nil, false, dupErr
	}

	res, err := imageproc.Process(p.Raw, imageproc.Options{
		MaxWidth:    p.Group.MaxWidth,
		MaxHeight:   p.Group.MaxHeight,
		Original:    p.User.UploadMode == "original",
		Variant:     p.User.VariantFormat,
		ResizeWidth: p.User.MaxImageWidth,
		// Site-wide, not per-user: keeping originals is a storage-budget
		// decision, and the bytes come out of the platform pool.
		KeepOriginal: s.uploadSettings().KeepOriginal,
		ThumbWidth:   p.User.ThumbWidth,
		ThumbFormat:  p.User.ThumbFormat,
	})
	if err != nil {
		return nil, false, err
	}

	total := res.TotalBytes()
	imageID := uuid.New()

	// Reserve quota before writing objects. Doing it the other way round means
	// a user over their limit still costs us the bytes.
	if p.OnPlatform {
		if err := quota.Consume(s.DB, p.User.ID, total, fmt.Sprintf("上传 %s", p.OrigName), &imageID); err != nil {
			if errors.Is(err, quota.ErrInsufficient) {
				return nil, false, localize(
					i18n.TCtx(ctx, "storage.quota", humanBytes(total), humanBytes(quota.Available(p.User))),
					quota.ErrInsufficient)
			}
			return nil, false, err
		}
	}
	// From here on, any failure must give the reservation back.
	refund := func() {
		if p.OnPlatform {
			if err := quota.Release(s.DB, p.User.ID, total, "上传失败回退", &imageID); err != nil {
				log.Printf("upload: quota refund failed for user %s: %v", p.User.ID, err)
			}
		}
	}

	primaryKey, err := s.newObjectKey(ctx, res.Primary.Ext)
	if err != nil {
		refund()
		return nil, false, err
	}
	shortCode, err := s.newShortCode(s.DB)
	if err != nil {
		refund()
		return nil, false, err
	}
	written := []string{}
	putObject := func(key string, out imageproc.Output) error {
		if err := p.Backend.Put(ctx, key, bytes.NewReader(out.Data), out.Size(), out.MIME); err != nil {
			return err
		}
		written = append(written, key)
		return nil
	}

	if err := putObject(primaryKey, res.Primary); err != nil {
		refund()
		return nil, false, localize(i18n.TCtx(ctx, "storage.write_failed", err.Error()), err)
	}
	for name, out := range res.Variants {
		if err := putObject(models.VariantKey(primaryKey, name), out); err != nil {
			// A missing derivative is survivable — the original is already up
			// and the queue can regenerate the rest — but a half-written set
			// with no row would be an orphan, so unwind and fail cleanly.
			cleanupObjects(ctx, p.Backend, written)
			refund()
			return nil, false, localize(i18n.TCtx(ctx, "storage.write_variant_failed", name, err.Error()), err)
		}
	}

	img := models.Image{
		ID:           imageID,
		UserID:       p.User.ID,
		ProfileID:    profileID,
		SHA256:       p.SHA,
		ObjectKey:    primaryKey,
		ShortCode:    shortCode,
		OrigName:     p.OrigName,
		MIME:         res.Primary.MIME,
		Ext:          res.Primary.Ext,
		ProcSig:      sig,
		Width:        res.Primary.Width,
		Height:       res.Primary.Height,
		SizeOrig:     int64(len(p.Raw)),
		SizeStored:   total,
		SizePrimary:  res.Primary.Size(),
		SizeVariants: res.VariantBytes(),
		SizeThumbs:   res.ThumbBytes(),
		Variants:     res.VariantNames(),
		BackupState:  models.BackupNone,
		Status:       models.ImageActive,
		UploadIP:     p.IP,
	}
	if err := s.DB.Create(&img).Error; err != nil {
		cleanupObjects(ctx, p.Backend, written)
		refund()
		return nil, false, err
	}
	s.afterCreate(&img)
	return &img, false, nil
}

// procSig 把决定主图字节的全部用户设置压成一个签名,作为秒传克隆的相容
// 判据:原样模式只有一种产物;优化模式的产物由转换目标与限宽共同决定。
// 缩略图策略故意不进签名——它只影响网格小图,为它放弃整份对象的复用不
// 值得。改动此函数会让存量签名全部失配(等同关闭一次秒传),慎重。
func procSig(u *models.User) string {
	if u.UploadMode == "original" {
		return "orig"
	}
	variant := u.VariantFormat
	if variant == "" {
		variant = "none"
	}
	return fmt.Sprintf("opt:%s:%d", variant, u.MaxImageWidth)
}

// cloneImage records a second reference to bytes that are already stored.
// Quota is still charged: otherwise re-uploading the same file would be a free
// way to keep a copy, and every user's "space used" would stop matching the
// pictures they can see. The saving is ours, not theirs.
func (s *Server) cloneImage(ctx context.Context, twin models.Image, p storeParams) (*models.Image, error) {
	imageID := uuid.New()
	if p.OnPlatform {
		if err := quota.Consume(s.DB, p.User.ID, twin.SizeStored, fmt.Sprintf("上传 %s", p.OrigName), &imageID); err != nil {
			if errors.Is(err, quota.ErrInsufficient) {
				return nil, localize(
					i18n.TCtx(ctx, "storage.quota", humanBytes(twin.SizeStored), humanBytes(quota.Available(p.User))),
					quota.ErrInsufficient)
			}
			return nil, err
		}
	}
	shortCode, err := s.newShortCode(s.DB)
	if err != nil {
		return nil, err
	}
	img := models.Image{
		ID:           imageID,
		UserID:       p.User.ID,
		ProfileID:    twin.ProfileID,
		SHA256:       twin.SHA256,
		ObjectKey:    twin.ObjectKey,
		ShortCode:    shortCode,
		OrigName:     p.OrigName,
		MIME:         twin.MIME,
		Ext:          twin.Ext,
		ProcSig:      twin.ProcSig,
		Width:        twin.Width,
		Height:       twin.Height,
		SizeOrig:     int64(len(p.Raw)),
		SizeStored:   twin.SizeStored,
		SizePrimary:  twin.SizePrimary,
		SizeVariants: twin.SizeVariants,
		SizeThumbs:   twin.SizeThumbs,
		Variants:     twin.Variants,
		BackupState:  twin.BackupState,
		Status:       models.ImageActive,
		UploadIP:     p.IP,
	}
	if err := s.DB.Create(&img).Error; err != nil {
		if p.OnPlatform {
			if rerr := quota.Release(s.DB, p.User.ID, twin.SizeStored, "秒传失败回退", &imageID); rerr != nil {
				log.Printf("upload: quota refund failed for user %s: %v", p.User.ID, rerr)
			}
		}
		return nil, err
	}
	return &img, nil
}

// afterCreate queues the work that was deliberately kept out of the request:
// replication to a backup bucket. AVIF 曾经也在这里入队——转换语义落地后
// 主图在上传时同步编码为目标格式,异步补变体的路径不复存在(原样模式下
// 转换设置整体不参与)。
func (s *Server) afterCreate(img *models.Image) {
	if s.Queue == nil {
		return
	}
	var backupCount int64
	s.DB.Model(&models.StorageProfile{}).
		Where("backup_of_id = ? AND status = ?", img.ProfileID, models.ProfileActive).
		Count(&backupCount)
	if backupCount > 0 {
		s.DB.Model(img).Update("backup_state", models.BackupPending)
		img.BackupState = models.BackupPending
		s.Queue.Submit(scheduler.JobBackup, img.ID)
	}
}

// readAndHash reads the upload fully while computing its SHA-256 in one pass.
func readAndHash(fh *multipart.FileHeader) ([]byte, string, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	h := sha256.New()
	buf := &bytes.Buffer{}
	buf.Grow(int(fh.Size))
	if _, err := io.Copy(io.MultiWriter(buf, h), f); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), hex.EncodeToString(h.Sum(nil)), nil
}

// cleanupObjects removes objects written during a failed upload. Best effort:
// a leaked object is wasted space, but a failed cleanup must not mask the
// original error.
func cleanupObjects(ctx context.Context, backend storage.Backend, keys []string) {
	// The request context may already be cancelled at this point — that's
	// exactly when cleanup matters most.
	ctx = context.WithoutCancel(ctx)
	for _, k := range keys {
		if err := backend.Delete(ctx, k); err != nil {
			log.Printf("upload: failed to clean up orphaned object %s: %v", k, err)
		}
	}
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// newObjectKey mints an unused key for a fresh upload.
//
// A random 12-character id collides with probability around 1e-4 across a
// billion images, which is small but not zero — and the consequence is not a
// failed upload, it is silently overwriting somebody else's picture. So the
// key is checked against the index before use and re-rolled on a hit.
//
// The check cannot be a unique constraint on the column: deduplicated uploads
// deliberately share one key across many rows, and a unique index would reject
// exactly the case the dedup path is built for.
func (s *Server) newObjectKey(ctx context.Context, ext string) (string, error) {
	now := time.Now()
	for attempt := 0; attempt < 5; attempt++ {
		key := models.ObjectKeyFor(now, ext)
		var n int64
		if err := s.DB.Model(&models.Image{}).Where("object_key = ?", key).Count(&n).Error; err != nil {
			return "", err
		}
		if n == 0 {
			return key, nil
		}
		log.Printf("upload: object key %s already taken, re-rolling", key)
	}
	return "", errors.New(i18n.TCtx(ctx, "upload.key_failed"))
}
