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

// aiKnownResolutions 是上游认识的全部档位。它和"这个用户能用哪些"是两件事:
// 前者不认识 → 客户端拼错了,400;认识但没资格 → 403。混成一件事就只能笼统
// 报错,客户端分不清该改参数还是该去关联账号。
var aiKnownResolutions = map[string]bool{"1k": true, "2k": true, "4k": true}

// aiFreeResolutions 是没关联 pic.bi 时能用的档位。
//
// 不放 4k:它在上游是最贵的一档,而这里发的是免费额度——放开等于让每个用户
// 每月按最高价烧 50 次。
var aiFreeResolutions = []string{"1k", "2k"}

// aiLinkedResolutions 是关联了 pic.bi 之后的档位。4k 在这里放开,因为花的
// 是用户自己在 pic.bi 充的钱,而 pic.bi 会按档位算真实价钱(1k/2k/4k =
// 1/2/4 分),没有理由替他省。
var aiLinkedResolutions = []string{"1k", "2k", "4k"}

// allowedResolutionsFor 是"这个用户能用哪些档位"的唯一答案。
//
// 唯一这件事是重点:/api/ai/status 报出去的清单、/generate 与 /edit 收进来的
// 校验,必须读同一个函数。分成两处的下场是界面上不给选 4k,而直接 POST
// resolution=4k 就能绕过去——原来的代码正是这样(校验不过就静默回落 1k,
// 等于永远不会拒绝)。
func allowedResolutionsFor(u *models.User) []string {
	if picbiLinked(u) {
		return aiLinkedResolutions
	}
	return aiFreeResolutions
}

// picbiLinked:这个人的额度能不能从 pic.bi 扣。两个条件都要——用户绑了,
// 且这个部署确实配了 partner 凭据。
func picbiLinked(u *models.User) bool {
	return u != nil && u.PicbiID != "" && aigen.RemoteEnabled()
}

// freeRes 与 aigen.freeResolution 同义,重复一份是因为 api 不该 import 那个
// 私有函数;两处都改才算改对(账本那一层才是真闸门)。
func freeRes(r string) bool {
	for _, x := range aiFreeResolutions {
		if x == r {
			return true
		}
	}
	return false
}

func resolutionAllowed(u *models.User, r string) bool {
	for _, x := range allowedResolutionsFor(u) {
		if x == r {
			return true
		}
	}
	return false
}

// pickResolution 校验档位,不合格就自己把错误写出去并返回 false。
//
// 空值仍然当 1k:客户端可以不传这个字段。但一个"传了却不被允许"的值绝不
// 静默回落——回落会让越权请求以成功的样子返回,用户以为自己出了 4k 的图。
func (s *Server) pickResolution(c *gin.Context, u *models.User, raw string) (string, bool) {
	r := strings.TrimSpace(raw)
	if r == "" {
		return "1k", true
	}
	// API token 会话只能用免费档。理由见 aigen.Begin 的 localOnly:那种令牌
	// 会被粘进第三方客户端,而解绑 pic.bi 只认网页会话——让它能花真钱、本人
	// 又撤不回来,这个组合不能存在。
	if auth.ViaToken(c) && !freeRes(r) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": i18n.T(c, "ai.resolution_needs_session"), "code": "resolution_needs_session"})
		return "", false
	}
	if !aiKnownResolutions[r] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": i18n.T(c, "ai.resolution_unknown"), "code": "resolution_unknown"})
		return "", false
	}
	if !resolutionAllowed(u, r) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": i18n.T(c, "ai.resolution_denied"), "code": "resolution_denied"})
		return "", false
	}
	return r, true
}

// aiBeginFailed 把 aigen.Begin 的错误翻成 HTTP。两个入口共用,免得一处补了
// pic.bi 的分支另一处忘了。
//
// pic.bi 不可达是 503 而不是 402:这两者绝不能混。402 的含义是"你没钱了,
// 去签到",而不可达的含义是"我们不知道你有没有钱"。把后者当前者报,下一步
// 就是有人提议"那不如退回免费额度"——那正是 pic.bi 每抖一次全站免费刷 4k 的
// 那个洞。
func (s *Server) aiBeginFailed(c *gin.Context, err error) {
	switch {
	case errors.Is(err, aigen.ErrRemoteUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": i18n.T(c, "ai.picbi_unreachable"), "code": "picbi_unreachable"})
	case errors.Is(err, aigen.ErrRemoteDenied):
		c.JSON(http.StatusForbidden, gin.H{
			"error": i18n.T(c, "ai.picbi_denied"), "code": "picbi_denied"})
	case errors.Is(err, aigen.ErrDailyLimit):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": i18n.T(c, "ai.daily_limit")})
	case errors.Is(err, aigen.ErrNoCredits):
		c.JSON(http.StatusPaymentRequired, gin.H{"error": i18n.T(c, "ai.no_credits")})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

type aiGenerateReq struct {
	Prompt     string `json:"prompt" binding:"required"`
	Size       string `json:"size"`
	Resolution string `json:"resolution"`
	// Purpose 说明这次生成要拿来做什么。空值是普通的文生图。
	//
	// 为什么是"用途"而不是让客户端自己传 model/background/output_format:
	// 那三个参数之间有硬约束(见 aiWatermarkPlan),而且其中两个是上游的
	// 实现细节。让客户端拼这三个值,等于把"哪个模型认哪个参数"这条会随上游
	// 变的知识复制到每一个客户端里,而其中一份拼错的表现是拿到一张不透明的
	// 图、没有任何报错。
	Purpose string `json:"purpose"`
}

// aiPurposeWatermark 是"给我画一枚水印"。
const aiPurposeWatermark = "watermark"

// aiGenPlan 是一次生成的全部参数决定,由用途一次算出。
type aiGenPlan struct {
	Model      string
	Size       string
	Resolution string
}

// aiWatermarkPlan 把水印这个用途要的参数写在一处。
//
//   - 模型不换:全站只用 gpt-image-2 这一个。它不认 background/output_format,
//     所以上游给回来的是一张**不透明**的图——透明底由客户端本机抠出来
//     (Vision 的前景分割,编辑器里那个"去背景"),不额外调一次上游、也不多花
//     一份钱。提示词里明写"纯白背景",正是为了让抠图好抠。
//   - 1:1 写死:水印是个角标,方形最好摆;非方形还要考虑锚点方向。
//   - 1k 写死:水印最终按画面宽度的百分之十几渲染,一张 4k 的 logo 缩到那个
//     尺寸,多出来的像素一个都用不上——而 4k 恰恰是最贵的一档。
//
// 1k 同时也是免费档,所以这条路不必过 pickResolution:那道闸拦的是"传了一个
// 自己没资格用的档位",而这里的档位根本不由客户端决定。
// aiWatermarkPrompt 把用户那一句话包成一条真正能出水印的提示词。
//
// 用户只该说"画什么"(山峰、猫头鹰、字母 G),剩下的不该由他操心——而那些约束
// 又恰恰是这件事成不成的关键,漏一条就得重生成一次、白花一次额度。
//
// 每一条都对应一个具体的失败:
//
//   - **纯白背景、无渐变无阴影**:透明底是本机用 Vision 的前景分割抠出来的,
//     它认的是"主体与背景分得开"。渐变底、投影、倒影都会让边缘糊成一片,
//     抠完带一圈灰边。这是整条约束里最要紧的一条。
//   - **粗线条、大色块、无细节**:水印最终按画面宽度的百分之十几渲染、
//     再压到 45% 不透明度。一枚在 1024 像素上好看的精细图标,到那个尺寸只剩
//     一团脏点。
//   - **居中、留白**:锚点贴边摆放,主体顶到画布边缘的话,缩放后会被切掉一角。
//   - **不要额外文字**:模型很爱自作主张加一行英文标语。水印上的小字在那个
//     尺寸下是不可读的噪点。但用户自己要求的字母或字要留——所以说的是"不要
//     上面没描述过的文字",不是"不要文字"。
//
// 用英文写,用户那句话原样嵌进去:图像模型对英文指令的服从度明显更高,而主体
// 描述用什么语言都能认。
func aiWatermarkPrompt(user string) string {
	return "A single logo mark, centered on the canvas: " + user + ".\n" +
		"Style: flat vector icon, bold simple shapes, thick strokes, " +
		"solid high-contrast silhouette, no fine detail, no texture, no gradient.\n" +
		"Background: one flat pure white background, completely empty, " +
		"no shadow, no reflection, no border, no frame, no mockup, no scene.\n" +
		"Composition: the mark fills about 70% of the canvas and is fully visible, " +
		"with clear empty margin on all four sides.\n" +
		"Do not add any text, letters, words or signature that was not described above."
}

func aiWatermarkPlan() aiGenPlan {
	return aiGenPlan{
		Model:      aigen.DefaultModel,
		Size:       "1:1",
		Resolution: "1k",
	}
}

// aiSubmitOpts 是递交给上游、却不进数据库的那几个参数。
//
// 不落库是因为没有任何路径会重新递交:提交只发生在 handler 里,对账器和
// 轮询拿着任务号问上游,不需要原始请求体。多开三列存的是一份永远不会被读的
// 拷贝,外加一次迁移。
type aiSubmitOpts struct {
	ImageURLs []string
	// UpstreamPrompt 覆盖发给上游的提示词,记录里存的那份不变。
	//
	// 两者分开是因为它们服务于不同的读者:记录给人看("我当时想要什么"),
	// 上游那份给模型看(还带着一堆约束)。混成一份的话,历史列表里会是一大段
	// 英文模板,而「抄这条描述再生成一次」会把模板塞回输入框——用户再也改不回
	// 自己那句话。
	UpstreamPrompt string
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
	out := gin.H{
		"enabled":     true,
		"credits":     bal.Credits,
		"used_today":  bal.UsedToday,
		"daily_limit": bal.DailyLimit,
		"monthly":     bal.Monthly,
		"remaining":   bal.Remaining(),
		// 余额的拆解。少了它,"本月余额 56 · 每月配 50"读起来就像数据错了
		// ——那 6 次是签到攒的,而界面从前无从说起。
		"monthly_left": bal.MonthlyLeft,
		"granted":      bal.Granted,
		"sizes":       []string{"1:1", "3:2", "2:3", "4:3", "3:4", "16:9", "9:16"},
		// 与两个提交入口共用同一个函数。界面上能选什么,后端就收什么。
		"resolutions":  allowedResolutionsFor(u),
		"picbi_linked": picbiLinked(u),
	}
	// 快过期的那批要单独说。额度有了有效期,却不告诉用户哪天作废,等于没有。
	if bal.NextExpiry != nil {
		out["next_expiry"] = bal.NextExpiry
		out["next_expiry_credits"] = bal.NextExpiryCredits
	}
	// 关联之后本地额度用完不等于不能生成了,界面得知道那边还剩多少,否则
	// remaining=0 会把入口整个藏掉。查不到就不报这个字段——它是锦上添花,
	// 不该让 pic.bi 抖一下就把整个状态接口拖垮。
	if picbiLinked(u) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		if n, err := aigen.RemoteBalance(ctx, u.PicbiID); err == nil {
			out["picbi_credits"] = n
		} else {
			log.Printf("ai: 查 pic.bi 余额失败 user=%s: %v", u.ID, err)
		}
	}
	c.JSON(http.StatusOK, out)
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
	// 认不出的用途一律拒绝,绝不当成普通生成。静默忽略的下场是:客户端以为
	// 自己要到了一张透明底的水印,拿回来的却是一张不透明的方图,而没有任何
	// 一处报过错。
	purpose := strings.TrimSpace(req.Purpose)
	if purpose != "" && purpose != aiPurposeWatermark {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": i18n.T(c, "ai.purpose_unknown"), "code": "purpose_unknown"})
		return
	}

	plan := aiGenPlan{Model: aigen.DefaultModel, Size: req.Size}
	if purpose == aiPurposeWatermark {
		plan = aiWatermarkPlan()
	} else {
		if !aiAllowedSizes[plan.Size] {
			plan.Size = "1:1"
		}
		r, ok := s.pickResolution(c, u, req.Resolution)
		if !ok {
			return
		}
		plan.Resolution = r
	}

	g := s.groupFor(u)
	gen := models.AIGeneration{
		Kind:       models.AIGenKindGenerate,
		Prompt:     prompt,
		Model:      plan.Model,
		Size:       plan.Size,
		Resolution: plan.Resolution,
		Credits:    1,
	}
	// 数今日、扣额度、落记录三步在一个事务里完成,理由见 aigen.Begin:
	// 记录不落库,日限就没有计数依据,并发提交会全部放行。
	if err := aigen.Begin(s.DB, u, g, &gen, auth.ViaToken(c)); err != nil {
		s.aiBeginFailed(c, err)
		return
	}

	var opts aiSubmitOpts
	if purpose == aiPurposeWatermark {
		opts.UpstreamPrompt = aiWatermarkPrompt(prompt)
	}
	s.submitAIGen(c, &gen, opts)
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
	resolution, ok2 := s.pickResolution(c, u, req.Resolution)
	if !ok2 {
		return
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
	if err := aigen.Begin(s.DB, u, g, &gen, auth.ViaToken(c)); err != nil {
		s.aiBeginFailed(c, err)
		return
	}

	s.submitAIGen(c, &gen, aiSubmitOpts{ImageURLs: urls})
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
//
// opts 里是那些不进数据库的上游参数(参考图地址、透明底)。它们由调用方算好
// 传进来,这里不再猜:同样是 gpt-image-1,一次普通生成和一次水印生成要的
// background 并不一样,从 gen.Model 反推只会推错一半。
func (s *Server) submitAIGen(c *gin.Context, gen *models.AIGeneration, opts aiSubmitOpts) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	upstream := gen.Prompt
	if opts.UpstreamPrompt != "" {
		upstream = opts.UpstreamPrompt
	}
	taskID, err := s.AIGen.Submit(ctx, aigen.Req{
		Prompt:     upstream,
		Model:      gen.Model,
		Size:       gen.Size,
		Resolution: gen.Resolution,
		ImageURLs:  opts.ImageURLs,
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
	if err := s.DB.Where("user_id = ? AND hidden = ?", u.ID, false).
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
	// 传整条记录,不是 (userID, credits):退到哪本账、退哪一笔,由扣费那一刻
	// 写死在这条记录上的 Ledger/SpendOpID 决定,而不是由用户此刻还绑没绑
	// pic.bi 决定。见 models.AIGeneration 上那几列的注释。
	if err := aigen.Refund(s.DB, gen); err != nil {
		// 本地账本下这是一次几乎不会失败的 UPDATE,但错误绝不能吞:接上
		// pic.bi 之后退款失败会变成常态,而那时丢掉的是用户充值的真钱。
		log.Printf("ai: 退款失败 gen=%s user=%s ledger=%s op=%s credits=%d: %v",
			gen.ID, gen.UserID, gen.Ledger, gen.SpendOpID, gen.Credits, err)
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

// DELETE /api/ai/generations/:id — 把一条生成记录从列表里去掉。
//
// 只置 Hidden,不删行。行是日限的计数单位(见 aigen.UsedToday),真删等于把
// 当天配额退回来;而且退款重试和对账都要扫这张表,删掉就是把没结清的账一起
// 抹了。用户看到的是"删除",系统里是"不再显示"——这两件事在这里必须分开。
//
// 只能删已经结束的记录。还在 charging/pending/running 的那条,额度已经扣了、
// 上游可能还在跑,让它从界面上消失就等于让一笔未结的账消失,用户既看不到进
// 度也看不到退款。
//
// image=1 时连同产出图一起删,走的是删图那条完整的路(软删 + 配额退还 +
// 清理任务),而不是就地写一遍 UPDATE。产出图归用户所有,他要不要留是他的事,
// 所以这是显式开关而不是默认行为。
func (s *Server) handleDeleteAIGeneration(c *gin.Context) {
	u := auth.MustUser(c)

	var gen models.AIGeneration
	if err := s.DB.Where("id = ? AND user_id = ?", c.Param("id"), u.ID).
		First(&gen).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": i18n.T(c, "ai.gen_not_found"), "code": "not_found"})
		return
	}
	if !gen.Status.IsTerminal() {
		c.JSON(http.StatusConflict, gin.H{
			"error": i18n.T(c, "ai.delete_in_flight"), "code": "in_flight",
		})
		return
	}

	// 先删图再隐藏记录。反过来的话,删图失败时记录已经不见了,用户再也找不到
	// 那张图的来处——而现在这个顺序下,删图失败就整个失败,界面上那条还在。
	alsoImage := c.Query("image") == "1"
	if alsoImage && gen.ImageID != nil {
		var img models.Image
		if err := s.DB.Where("id = ? AND user_id = ? AND deleted_at IS NULL",
			*gen.ImageID, u.ID).First(&img).Error; err == nil {
			if err := s.softDeleteImage(&img); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		// 查不到就当已经删过了:目的是"这张图不在了",它已经不在了。
	}

	if err := s.DB.Model(&models.AIGeneration{}).
		Where("id = ? AND user_id = ?", gen.ID, u.ID).
		Update("hidden", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "image_deleted": alsoImage && gen.ImageID != nil})
}
