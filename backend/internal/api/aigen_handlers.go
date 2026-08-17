package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/aigen"
	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/i18n"
	"github.com/gentpan/openimg/backend/internal/imageproc"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/scheduler"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// aiGenMaxDownload 兜住上游返回的图。4K 档的 PNG 可以到十几兆,给足余量,
// 真正的上限仍由用户组的单文件大小在入库时把关。
const aiGenMaxDownload = 32 << 20

var aiAllowedSizes = map[string]bool{
	"1:1": true, "3:2": true, "2:3": true, "4:3": true, "3:4": true,
	"16:9": true, "9:16": true, "2:1": true, "1:2": true,
}

// aiAllowedResolutions 只放开 1k/2k。4k 在上游是最贵的档,而这里是免费额度
// ——放开它等于让每个用户每月按最高价烧 50 次。
var aiAllowedResolutions = map[string]bool{"1k": true, "2k": true}

type aiGenerateReq struct {
	Prompt     string `json:"prompt" binding:"required"`
	Size       string `json:"size"`
	Resolution string `json:"resolution"`
}

type aiEditReq struct {
	Prompt string `json:"prompt"`
	// ImageIDs 收字符串,解析放到后面自己做,理由见 parseSourceIDs。
	ImageIDs   []string `json:"image_ids"`
	Size       string   `json:"size"`
	Resolution string   `json:"resolution"`
}

// GET /api/ai/status — 这个部署有没有开 AI，以及我还能生成几次。
func (s *Server) handleAIStatus(c *gin.Context) {
	u := auth.MustUser(c)
	if !s.AIGen.Enabled() {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}
	g := s.groupFor(u)
	bal, err := aigen.Current(s.DB, u, g)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"enabled":     true,
		"credits":     bal.Credits,
		"used_today":  bal.UsedToday,
		"daily_limit": bal.DailyLimit,
		"monthly":     bal.Monthly,
		"remaining":   bal.Remaining(),
		"sizes":       []string{"1:1", "3:2", "2:3", "4:3", "3:4", "16:9", "9:16"},
		"resolutions": []string{"1k", "2k"},
	})
}

// POST /api/ai/generate — 递交一次生成。
//
// 同步部分只做到"拿到上游任务号"就返回:生成要几十秒,把请求挂在那里等,
// 代理层先超时,用户也不知道发生了什么。剩下的交给队列轮询,客户端查
// /api/ai/generations 看进度。
func (s *Server) handleAIGenerate(c *gin.Context) {
	u := auth.MustUser(c)
	if !s.AIGen.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": i18n.T(c, "ai.disabled")})
		return
	}
	if !u.EmailVerified {
		c.JSON(http.StatusForbidden, gin.H{"error": i18n.T(c, "upload.verify_email")})
		return
	}

	var req aiGenerateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "ai.prompt_required")})
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "ai.prompt_required")})
		return
	}
	if len([]rune(prompt)) > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "ai.prompt_too_long")})
		return
	}
	size := req.Size
	if !aiAllowedSizes[size] {
		size = "1:1"
	}
	resolution := req.Resolution
	if !aiAllowedResolutions[resolution] {
		resolution = "1k"
	}

	g := s.groupFor(u)
	gen := models.AIGeneration{
		Kind:       models.AIGenKindGenerate,
		Prompt:     prompt,
		Model:      aigen.DefaultModel,
		Size:       size,
		Resolution: resolution,
		Credits:    1,
	}
	// 数今日、扣额度、落记录三步在一个事务里完成,理由见 aigen.Begin:
	// 记录不落库,日限就没有计数依据,并发提交会全部放行。
	if err := aigen.Begin(s.DB, u, g, &gen); err != nil {
		switch {
		case errors.Is(err, aigen.ErrDailyLimit):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": i18n.T(c, "ai.daily_limit")})
		case errors.Is(err, aigen.ErrNoCredits):
			c.JSON(http.StatusPaymentRequired, gin.H{"error": i18n.T(c, "ai.no_credits")})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	s.submitAIGen(c, &gen, nil)
}

// POST /api/ai/edit — 拿自己图库里的图去改。
//
// 与 /api/ai/generate 是同一件事的两种入口:同一把额度闸、同一条轮询、同一
// 条退款路径、同一条入库流水线,差别只在提交给上游时多带几张源图的地址。所以
// 这里只负责"把源图校验成一串公开 URL",剩下的交给 submitAIGen。
func (s *Server) handleAIEdit(c *gin.Context) {
	u := auth.MustUser(c)
	if !s.AIGen.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": i18n.T(c, "ai.disabled")})
		return
	}
	if !u.EmailVerified {
		c.JSON(http.StatusForbidden, gin.H{"error": i18n.T(c, "upload.verify_email")})
		return
	}

	var req aiEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "ai.prompt_required")})
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "ai.prompt_required")})
		return
	}
	if len([]rune(prompt)) > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "ai.prompt_too_long")})
		return
	}

	ids, ok := parseSourceIDs(req.ImageIDs)
	if !ok {
		// 带一个机器可读的 code:客户端要区分"没选图"和"图不在了"才能给出
		// 不同的说法,而 error 里是已经翻译过的人话,拿它去 contains 匹配
		// 一换语言就失效。
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "ai.no_source"), "code": "no_source"})
		return
	}
	urls, err := s.sourceImageURLs(u.ID, ids)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": i18n.T(c, "ai.source_missing"), "code": "source_missing"})
		return
	}

	// size 与文生图不同:留空是有意义的取值,表示"跟着输入图走"。只有用户
	// 明确点了某个比例才传给上游,乱填的照样落回空。
	size := req.Size
	if !aiAllowedSizes[size] {
		size = ""
	}
	resolution := req.Resolution
	if !aiAllowedResolutions[resolution] {
		resolution = "1k"
	}

	g := s.groupFor(u)
	gen := models.AIGeneration{
		Kind:       models.AIGenKindEdit,
		SourceIDs:  models.JoinSourceIDs(ids),
		Prompt:     prompt,
		Model:      aigen.DefaultModel,
		Size:       size,
		Resolution: resolution,
		Credits:    1,
	}
	if err := aigen.Begin(s.DB, u, g, &gen); err != nil {
		switch {
		case errors.Is(err, aigen.ErrDailyLimit):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": i18n.T(c, "ai.daily_limit")})
		case errors.Is(err, aigen.ErrNoCredits):
			c.JSON(http.StatusPaymentRequired, gin.H{"error": i18n.T(c, "ai.no_credits")})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	s.submitAIGen(c, &gen, urls)
}

// aiEditMaxSources 是一次修图能带几张源图。上游允许 16 张,这里收到 4:一次
// 生成只扣 1 点,源图越多上游算得越贵,而 4 张已经够覆盖"合成/换背景/参考
// 风格"这些实际用法。
const aiEditMaxSources = 4

// parseSourceIDs 校验数量并把字符串解析成 ID。
//
// 收字符串而不是直接绑成 []uuid.UUID:后者一旦有一个 ID 格式不对,整个
// ShouldBindJSON 就失败,只能笼统地报"提示词有问题",而真正的原因在图上。
func parseSourceIDs(raw []string) ([]uuid.UUID, bool) {
	if len(raw) == 0 || len(raw) > aiEditMaxSources {
		return nil, false
	}
	ids := make([]uuid.UUID, 0, len(raw))
	seen := map[uuid.UUID]bool{}
	for _, s := range raw {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			// 解析不出来的 ID 一定不在这个人的图库里,和"图不存在"同类,
			// 但这里还分不出该报哪个,交给调用方按 no_source 处理。
			return nil, false
		}
		if seen[id] {
			continue // 同一张图传两遍没有意义,也白白抬高上游成本
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, false
	}
	return ids, true
}

// sourceImageURLs 把源图 ID 换成公开 URL,顺带完成归属校验。
//
// 归属这一步不能省:少了 user_id 这个条件,任何人只要猜到一个 ID 就能把别人
// 的图丢给上游去改。查询里同时排除已删除和被封禁的——它们的对象要么已经不在,
// 要么本来就不该再被取出来。
//
// URL 用 decorate 算,不自己拼:BYOS 的图挂在用户自己的域名下,平台域名拼出来
// 的地址上游根本取不到。decorate 已经把 profile 的 PublicBaseURL、KeyPrefix
// 和站点回退基址那套规则处理完了。
func (s *Server) sourceImageURLs(userID uuid.UUID, ids []uuid.UUID) ([]string, error) {
	var imgs []models.Image
	if err := s.DB.Where("id IN ? AND user_id = ? AND deleted_at IS NULL AND status <> ?",
		ids, userID, models.ImageBlocked).Find(&imgs).Error; err != nil {
		return nil, err
	}
	byID := map[uuid.UUID]imageOut{}
	for _, out := range s.decorate(imgs) {
		byID[out.ID] = out
	}

	urls := make([]string, 0, len(ids))
	for _, id := range ids {
		out, ok := byID[id]
		if !ok {
			return nil, errAISourceMissing
		}
		// 上游要去取这个地址,不是绝对 URL 就没得取。私有部署把
		// PUBLIC_BASE_URL 配成了内网地址时会落在这里,报错好过让上游
		// 拿着半截路径失败几十秒。
		if !strings.HasPrefix(out.URL, "http://") && !strings.HasPrefix(out.URL, "https://") {
			return nil, errAISourceMissing
		}
		urls = append(urls, out.URL)
	}
	return urls, nil
}

var errAISourceMissing = errors.New("ai: 源图不存在或不属于此用户")

// submitAIGen 是两个入口共用的后半段:递交上游、存任务号、入队轮询。
//
// gen 必须已经过 aigen.Begin(额度已扣、记录已落库)。失败一律走 failAIGen,
// 退款与状态落库因此只有一条路径,不会因为多一个入口就多一份要维护的退款逻辑。
func (s *Server) submitAIGen(c *gin.Context, gen *models.AIGeneration, imageURLs []string) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	taskID, err := s.AIGen.Submit(ctx, aigen.Req{
		Prompt:     gen.Prompt,
		Model:      gen.Model,
		Size:       gen.Size,
		Resolution: gen.Resolution,
		ImageURLs:  imageURLs,
	})
	if err != nil {
		// 没递交成功就没花上游的钱。走统一的失败路径,让退款和状态落库
		// 只发生一次。
		_ = s.failAIGen(gen, err.Error())
		c.JSON(http.StatusBadGateway, gin.H{"error": i18n.T(c, "ai.submit_failed", err.Error())})
		return
	}

	if err := s.DB.Model(gen).Updates(map[string]any{
		"task_id": taskID,
		"status":  models.AIGenPending,
	}).Error; err != nil {
		// 任务号没存住就无从轮询,但上游已经在跑了。标失败并退款,由对账器
		// 兜住剩下的——总好过留一条查不到进度的记录。
		_ = s.failAIGen(gen, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	gen.TaskID, gen.Status = taskID, models.AIGenPending

	s.queueAIPoll(gen.ID)
	c.JSON(http.StatusAccepted, gin.H{"generation": gen})
}

// GET /api/ai/generations — 最近的生成记录,客户端靠它看进度。
func (s *Server) handleAIGenerations(c *gin.Context) {
	u := auth.MustUser(c)
	var list []models.AIGeneration
	if err := s.DB.Where("user_id = ?", u.ID).
		Order("created_at DESC").Limit(30).Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 顺带把记录牵扯到的图片带上,客户端不用再查一次图库。产出图与修图的
	// 源图放进同一个 map:两者都只是"一张图",分成两个字段只会让客户端多写
	// 一遍相同的取用代码。已删除的源图查不出来,客户端按缺失渲染即可。
	ids := make([]uuid.UUID, 0, len(list))
	for _, g := range list {
		if g.ImageID != nil {
			ids = append(ids, *g.ImageID)
		}
		ids = append(ids, g.SourceIDList()...)
	}
	images := map[string]imageOut{}
	if len(ids) > 0 {
		var imgs []models.Image
		s.DB.Where("id IN ? AND user_id = ? AND deleted_at IS NULL", ids, u.ID).Find(&imgs)
		for _, out := range s.decorate(imgs) {
			images[out.ID.String()] = out
		}
	}
	c.JSON(http.StatusOK, gin.H{"generations": list, "images": images})
}

// queueAIPoll 用会丢的 Submit 而不是 SubmitWait,是有意的分工:主路径要快,
// 不能因为队列满了就把 HTTP 请求挂在那里;而丢掉的那一条由对账器在下一轮
// 巡检时捡回来(见 aigen_reconcile.go)。代价是极端情况下用户要多等一会儿才
// 看到进度,换来的是队列拥塞时接口仍然响应。
func (s *Server) queueAIPoll(id uuid.UUID) {
	if s.Queue == nil {
		return
	}
	s.Queue.Submit(scheduler.JobAIPoll, id)
}

// jobAIPoll 跟进一次生成:轮上游,成了就把图downloading进正常的上传流水线。
//
// 轮询而不是等回调:上游没有 webhook,而且回调要求本服务可被公网直达,自建
// 部署未必满足。轮询的代价是几次 HTTP,换来的是任何部署都能用。
func (s *Server) jobAIPoll(ctx context.Context, job scheduler.Job) error {
	var gen models.AIGeneration
	// Job 只带一个 UUID 字段,AI 任务用它承载生成记录 ID。
	if err := s.DB.First(&gen, "id = ?", job.ImageID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if gen.Status.IsTerminal() {
		return nil
	}
	if gen.Status == models.AIGenPending {
		s.DB.Model(&gen).Update("status", models.AIGenRunning)
	}

	// 在这个 job 内部轮询到出结果为止:队列的单任务超时是 5 分钟,而一次
	// 生成通常几十秒。超时了就让队列按失败重试,记录仍在,不会丢。
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	for {
		res, err := s.AIGen.Poll(ctx, gen.TaskID)
		if err != nil {
			var se *aigen.StatusError
			if errors.As(err, &se) && !se.Retryable() {
				return s.failAIGen(&gen, err.Error())
			}
			return err // 交给队列重试
		}
		if res.Done {
			if res.Error != "" {
				return s.failAIGen(&gen, res.Error)
			}
			return s.completeAIGen(ctx, &gen, res.URL)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// failAIGen 终结一次生成并退还额度。今日次数不退,它是防滥用的闸门。
//
// 顺序是先抢状态再退款,不能反。反过来的话:退款成功、随后那次状态落库因为
// 连接抖动失败 → handler 返回错误 → 队列重投 → jobAIPoll 顶上的终态检查因为
// 状态压根没写成而拦不住 → 再退一次,最多退到重试次数用尽。条件更新放在前面,
// 只有抢到状态转换的那一次才有资格退款。
func (s *Server) failAIGen(gen *models.AIGeneration, msg string) error {
	now := time.Now()
	res := s.DB.Model(&models.AIGeneration{}).
		Where("id = ? AND status NOT IN ?", gen.ID,
			[]models.AIGenStatus{models.AIGenCompleted, models.AIGenFailed}).
		Updates(map[string]any{
			"status":  models.AIGenFailed,
			"error":   msg,
			"done_at": &now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return nil // 已经被别的路径终结过了
	}
	if err := aigen.Refund(s.DB, gen.UserID, gen.Credits); err != nil {
		// 本地账本下这是一次几乎不会失败的 UPDATE,但错误绝不能吞:接上
		// pic.bi 之后退款失败会变成常态,而那时丢掉的是用户充值的真钱。
		log.Printf("ai: 退款失败 gen=%s user=%s credits=%d: %v",
			gen.ID, gen.UserID, gen.Credits, err)
	}
	gen.Status = models.AIGenFailed
	return nil
}

// completeAIGen 把生成的图下下来,走与手动上传完全相同的那条流水线。
//
// 复用而不是另写一条:去重、变体、缩略图、短链、备份、配额记账全在里面,
// 另开一条只会得到一张"少了一半功能的图"。从入库那一刻起,它就是一张普通
// 图片。
func (s *Server) completeAIGen(ctx context.Context, gen *models.AIGeneration, url string) error {
	raw, err := s.AIGen.Download(ctx, url, aiGenMaxDownload)
	if err != nil {
		return s.failAIGen(gen, err.Error())
	}
	ext, err := imageproc.Detect(raw)
	if err != nil {
		return s.failAIGen(gen, err.Error())
	}

	var user models.User
	if err := s.DB.First(&user, "id = ?", gen.UserID).Error; err != nil {
		return err
	}
	group := s.groupFor(&user)
	backend, profile, err := s.Storage.Resolve(ctx, &user)
	if err != nil {
		return s.failAIGen(gen, err.Error())
	}

	img, _, err := s.storeUpload(ctx, storeParams{
		User:       &user,
		Group:      &group,
		Backend:    backend,
		Profile:    profile,
		OnPlatform: profile == nil || profile.IsPlatform(),
		Raw:        raw,
		SHA:        hex.EncodeToString(sha256Sum(raw)),
		Ext:        ext,
		OrigName:   aiFileName(gen.Prompt, ext),
		IP:         "",
	})
	if err != nil {
		return s.failAIGen(gen, err.Error())
	}

	now := time.Now()
	return s.DB.Model(gen).Updates(map[string]any{
		"status":   models.AIGenCompleted,
		"image_id": img.ID,
		"done_at":  &now,
	}).Error
}

func sha256Sum(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

// aiFileName 用提示词开头给文件起个能认的名字。图库里一排 "ai.png" 谁也
// 分不清哪张是哪张。
func aiFileName(prompt, ext string) string {
	r := []rune(strings.TrimSpace(prompt))
	if len(r) > 24 {
		r = r[:24]
	}
	name := strings.Map(func(c rune) rune {
		switch c {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', '\n', '\r', '\t':
			return '-'
		}
		return c
	}, string(r))
	if strings.TrimSpace(name) == "" {
		name = "ai"
	}
	return name + "." + ext
}
