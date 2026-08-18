package aigen

import (
	"strings"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"time"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNoCredits  = errors.New("aigen: 生成次数已用完")
	ErrDailyLimit = errors.New("aigen: 今日生成次数已达上限")
)

// Balance 是一个用户当下能用多少次的完整答案。
type Balance struct {
	Credits    int `json:"credits"`
	UsedToday  int `json:"used_today"`
	DailyLimit int `json:"daily_limit"`
	Monthly    int `json:"monthly"`
}

// Remaining 是这一刻真正还能生成几次:余额与今日剩余里更小的那个。
func (b Balance) Remaining() int {
	left := b.DailyLimit - b.UsedToday
	if left < 0 {
		left = 0
	}
	if b.Credits < left {
		return b.Credits
	}
	return left
}

func monthKey(t time.Time) string { return t.UTC().Format("2006-01") }

// EnsureMonthly 按需补发当月额度。
//
// 没有定时任务:这个项目刻意不跑 cron(见 scheduler 的注释),所以月度发放
// 在"用到的时候"结算——发现记录里的月份不是本月,就把余额重置为本月的量。
// 重置而不是累加:每月 50 张是"每月的配给"而不是"攒着不过期的存货",否则
// 半年不用的账号会攒出 300 张的突袭额度。签到送的次数在本月内有效,同理。
func EnsureMonthly(db *gorm.DB, u *models.User, g models.UserGroup) error {
	now := monthKey(time.Now())
	if u.AICreditsMonth == now {
		return nil
	}
	u.AICredits = g.AIMonthly
	u.AICreditsMonth = now
	return db.Model(&models.User{}).Where("id = ?", u.ID).
		Updates(map[string]any{
			"ai_credits":       u.AICredits,
			"ai_credits_month": u.AICreditsMonth,
		}).Error
}

// UsedToday 数今天已经提交过几次。
//
// 按提交计而不是按成功计:上游那一次调用无论成没成都花掉了,而且按成功计
// 的话,一个反复失败的提示词可以无限次撞上游。失败会退还余额(见 Refund),
// 但今日次数不退——这正是它作为"闸门"的意义。
//
// 只数本地账本的行,与 beginLocal 里那个计数保持同一个口径:日限管的是免费
// 配给,pic.bi 那边花的是用户自己的钱。两处口径要是不一样,界面上显示的
// "今天还能几次"就会和实际能不能提交对不上。
func UsedToday(db *gorm.DB, userID uuid.UUID) (int, error) {
	start := time.Now().UTC().Truncate(24 * time.Hour)
	var n int64
	err := db.Model(&models.AIGeneration{}).
		Where("user_id = ? AND created_at >= ? AND ledger <> ?",
			userID, start, models.AIGenLedgerPicbi).
		Count(&n).Error
	return int(n), err
}

func Current(db *gorm.DB, u *models.User, g models.UserGroup) (Balance, error) {
	if err := EnsureMonthly(db, u, g); err != nil {
		return Balance{}, err
	}
	used, err := UsedToday(db, u.ID)
	if err != nil {
		return Balance{}, err
	}
	return Balance{
		Credits:    u.AICredits,
		UsedToday:  used,
		DailyLimit: g.AIDaily,
		Monthly:    g.AIMonthly,
	}, nil
}

// Reserve 在提交给上游之前先把次数扣掉。
//
// 先扣后用:上游那一次调用真的花了钱,先调再扣会让并发请求越过余额。扣不
// 动就直接拒,不进上游。
func Reserve(db *gorm.DB, u *models.User, g models.UserGroup, credits int) error {
	if credits <= 0 {
		credits = 1
	}
	if err := EnsureMonthly(db, u, g); err != nil {
		return err
	}
	used, err := UsedToday(db, u.ID)
	if err != nil {
		return err
	}
	if g.AIDaily > 0 && used >= g.AIDaily {
		return ErrDailyLimit
	}

	// 条件更新兼作并发闸门:两个请求同时进来,只有一个能把余额从 n 减到
	// n-1,另一个 RowsAffected 为 0。
	res := db.Model(&models.User{}).
		Where("id = ? AND ai_credits >= ?", u.ID, credits).
		UpdateColumn("ai_credits", gorm.Expr("ai_credits - ?", credits))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNoCredits
	}
	u.AICredits -= credits
	return nil
}

// Begin 开一次生成:先花本地的免费次数,花光了才去动 pic.bi 的钱。
//
// 顺序不能反。本地那份是签到、月度发放这些"送"出来的次数,而 pic.bi 的分是
// 用户在那边真金白银充的、还能在 pic.bi 站内消费。允许免费次数换成 pic.bi 的
// 分,就是给这个站开了一台免费铸币机。
//
// 分流放在这里而不是放到 handler:调用点只说"开一次生成",走哪本账是这个包
// 自己的事。散到调用点去判断,迟早有一处判得和别处不一样。
//
// gen 需预先填好 Prompt/Model/Size/Resolution;返回时它已入库,状态为
// charging,Ledger/PicbiUserID/SpendOpID/Credits 都已定格。
// localOnly 为真时这次生成只许花免费额度,绝不动 pic.bi。
//
// 它是给 API token 会话用的:那种令牌常年有效、没有 scope、会被粘进 PicGo
// 这类第三方客户端,而解绑 pic.bi 却只认网页 cookie 会话。让一枚上传令牌
// 能花掉用户充值的真钱、本人还撤不回来,这个组合不能存在。
func Begin(db *gorm.DB, u *models.User, g models.UserGroup, gen *models.AIGeneration, localOnly bool) error {
	if gen.Credits <= 0 {
		gen.Credits = 1
	}
	// 高于免费档的分辨率**不许走本地账本**。
	//
	// 免费额度是按"次"发的,一次就是一点,不认分辨率;而上游按档位真金白银
	// 地收(1k/2k/4k = 1/2/4)。把 4k 放进本地账本,等于关联一个 pic.bi 账号
	// ——不用充一分钱——就能拿每月 50 次免费额度全部按最高价烧,差价由本站
	// 的 Apimart token 承担。
	//
	// 未关联的人根本到不了这里:pickResolution 早就 403 拦掉了。所以这一步
	// 只可能命中"已关联且选了高档位"的人,而他们本来就该花自己的钱。
	if !freeResolution(gen.Resolution) {
		if localOnly || u == nil || u.PicbiID == "" || remote == nil || !remote.Enabled() {
			// 理论上到不了(pickResolution 已拦),但账本这一层不依赖上游的
			// 校验:漏一次就是免费送出去最贵的一档。
			return ErrNoCredits
		}
		return beginRemote(db, u, gen)
	}

	localErr := beginLocal(db, u, g, gen)
	if localOnly {
		return localErr
	}
	useRemote, err := chooseLedger(u, remote, localErr)
	if !useRemote {
		return err
	}
	return beginRemote(db, u, gen)
}

// freeResolution:这一档能不能用免费额度买单。
//
// 与 api 层的 aiFreeResolutions 是同一件事的两处表述,但**这里才是闸门**:
// 界面上不给选只是不给选,真正拦住的是账本这一层。两处都改才算改对。
func freeResolution(r string) bool {
	switch strings.TrimSpace(strings.ToLower(r)) {
	case "", "1k", "2k":
		return true
	}
	return false
}

// chooseLedger 决定本地这条路走不通之后还有没有下一条。
//
// 单独拎出来是为了能不带数据库地测:分流规则是这次改动里最容易出错、后果也
// 最贵的一段("未关联的人被扣了别人的钱""关联的人被拦在免费额度上"),它值得
// 有一组能直接跑的用例。
//
// 返回 (走远程?, 该往上报的错)。注意本地额度不足时报出去的仍是本地那个错误
// ——用户没绑 pic.bi,告诉他"pic.bi 不可用"毫无意义。
func chooseLedger(u *models.User, r Remote, localErr error) (bool, error) {
	if localErr == nil {
		return false, nil // 本地够用,根本不必惊动 pic.bi
	}
	// 只有"额度用完了"这两种才有下一条路。数据库出错、月度补发失败之类
	// 一律原样上报:那不是缺钱,是这台机器有问题,拿用户的钱去补是错的。
	if !errors.Is(localErr, ErrNoCredits) && !errors.Is(localErr, ErrDailyLimit) {
		return false, localErr
	}
	if u == nil || u.PicbiID == "" {
		return false, localErr
	}
	if r == nil || !r.Enabled() {
		return false, localErr
	}
	return true, nil
}

// beginLocal 是本地账本的那一半:在同一个事务里数今日、扣额度、落记录。
//
// 这三步必须原子,否则日限拦不住任何东西。UsedToday 数的是 ai_generations
// 的行数,而这行原本要等上游递交成功才写——中间隔着一次几十秒的 HTTP。同时
// 提交的请求会全部读到同一个旧计数、全部通过检查,日限形同虚设(只剩月额度
// 那个条件更新兜底,后果从"今天 5 张"退化成"一次烧光整月")。
//
// 用用户行的排他锁做串行点:粒度正好是"同一个用户的并发提交",既不会拖累
// 别人,也不需要 INSERT...SELECT 那种读起来费劲的条件插入。
//
// 失败时事务整个回滚,一行都不留——上面的分流因此可以放心地接着试 pic.bi。
func beginLocal(db *gorm.DB, u *models.User, g models.UserGroup, gen *models.AIGeneration) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// 锁住用户行。之后的数数与扣减都在这把锁里,同一用户的第二个请求
		// 会等在这里,拿到的是已经包含前一次的计数。
		var locked models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&locked, "id = ?", u.ID).Error; err != nil {
			return err
		}
		if err := ensureMonthlyTx(tx, &locked, g); err != nil {
			return err
		}

		if g.AIDaily > 0 {
			start := time.Now().UTC().Truncate(24 * time.Hour)
			var used int64
			// 只数本地账本的行。日限是"每月配给别被一天烧光"的闸门,而
			// pic.bi 那边花的是用户自己充的钱,不该反过来啃掉他的免费额度
			// ——否则付费用生成几次,当天的免费次数就没了。
			// 条件写 <> 'picbi' 而不是 = 'local':存量行这一列是空串。
			if err := tx.Model(&models.AIGeneration{}).
				Where("user_id = ? AND created_at >= ? AND ledger <> ?",
					u.ID, start, models.AIGenLedgerPicbi).
				Count(&used).Error; err != nil {
				return err
			}
			if int(used) >= g.AIDaily {
				return ErrDailyLimit
			}
		}

		if locked.AICredits < gen.Credits {
			return ErrNoCredits
		}
		if err := tx.Model(&models.User{}).Where("id = ?", u.ID).
			UpdateColumn("ai_credits", gorm.Expr("ai_credits - ?", gen.Credits)).Error; err != nil {
			return err
		}

		gen.UserID = u.ID
		gen.Status = models.AIGenCharging
		gen.Ledger = models.AIGenLedgerLocal
		gen.PicbiUserID = ""
		gen.SpendOpID = ""
		if err := tx.Create(gen).Error; err != nil {
			return err
		}

		// 事务提交后调用方手里的 u 才该反映新余额。
		u.AICredits = locked.AICredits - gen.Credits
		u.AICreditsMonth = locked.AICreditsMonth
		return nil
	})
}

// beginRemote 是 pic.bi 账本的那一半。
//
// 这里没有事务,也不该有:中间隔着一次跨服务的 HTTP,把它包进事务就等于在
// 对端超时的十秒里一直握着数据库的锁。原子性由别的东西保证——幂等键。
//
// 顺序是"先落库再扣钱":记录带着自己的 ID 先进去,幂等键就从这个 ID 算出来。
// 反过来先扣钱的话,扣完崩了就得到一笔谁也认不出来的流水;而现在最坏的情况
// 是一条 charging 且没有 SpendOpID 的孤儿行,对账器认得它,退款逻辑也知道
// 这种行没有东西可退。
func beginRemote(db *gorm.DB, u *models.User, gen *models.AIGeneration) error {
	if gen.ID == uuid.Nil {
		gen.ID = uuid.New()
	}
	gen.UserID = u.ID
	gen.Status = models.AIGenCharging
	gen.Ledger = models.AIGenLedgerPicbi
	gen.PicbiUserID = u.PicbiID
	gen.SpendOpID = ""
	// 先按 0 落库:该扣几点是 pic.bi 算的,这一刻还不知道。写一个猜的数
	// 更糟——退款读的就是这一列。
	gen.Credits = 0
	if err := db.Create(gen).Error; err != nil {
		return err
	}

	ctx, cancel := remoteCtx()
	defer cancel()
	opID, credits, err := remote.Spend(ctx, gen.PicbiUserID, gen.Model, gen.Resolution,
		spendKey(gen.ID), Describe(gen.Prompt))
	if err != nil {
		// 这里分两种失败,差别是钱到底动没动。
		//
		// 对端明确拒绝(余额不足、没授权、参数不合法)= 确定没扣。留着这行
		// 只会被对账器当成"扣了费没递交"去退一笔不存在的款,删掉。
		//
		// 超时、5xx、连不上 = 结果未知。请求很可能已经落到 pic.bi 上并且扣
		// 成了,只是回执没回来。这种行绝不能删——删了就再没有任何东西指向
		// 那笔钱。让它停在 charging 且没有 TaskID,对账器认得这个形状
		// (见 models.AIGenCharging 的注释),会走到退款那一步去把它捞回来。
		if definitelyNotCharged(err) {
			if delErr := db.Delete(&models.AIGeneration{}, "id = ?", gen.ID).Error; delErr != nil {
				log.Printf("picbi: 扣费被拒后清理记录也失败 gen=%s user=%s: %v", gen.ID, u.ID, delErr)
			}
		} else {
			log.Printf("picbi: 扣费结果未知,记录留在 charging 等对账 gen=%s picbi_user=%s: %v",
				gen.ID, gen.PicbiUserID, err)
		}
		return mapRemoteErr(err)
	}

	if err := db.Model(&models.AIGeneration{}).Where("id = ?", gen.ID).
		Updates(map[string]any{"credits": credits, "spend_op_id": opID}).Error; err != nil {
		// 钱扣掉了、流水号没存住 —— 这条记录以后永远退不了款。立刻原路退回,
		// 退不掉就把三要素大声记下来,人工能顺着 op_id 找回去。
		rctx, rcancel := remoteCtx()
		defer rcancel()
		if rerr := remote.Refund(rctx, opID, refundKey(gen.ID)); rerr != nil {
			log.Printf("picbi: 流水号落库失败且退款失败 gen=%s picbi_user=%s op=%s credits=%d: 落库 %v / 退款 %v",
				gen.ID, gen.PicbiUserID, opID, credits, err, rerr)
		} else {
			log.Printf("picbi: 流水号落库失败,已原路退回 gen=%s op=%s credits=%d: %v",
				gen.ID, opID, credits, err)
		}
		db.Delete(&models.AIGeneration{}, "id = ?", gen.ID)
		return err
	}
	gen.Credits, gen.SpendOpID = credits, opID
	return nil
}

// ensureMonthlyTx 是 EnsureMonthly 的事务内版本。逻辑一致,只是复用外面
// 传进来的 tx,好让补发与扣减落在同一把锁里。
func ensureMonthlyTx(tx *gorm.DB, u *models.User, g models.UserGroup) error {
	now := monthKey(time.Now())
	if u.AICreditsMonth == now {
		return nil
	}
	u.AICredits = g.AIMonthly
	u.AICreditsMonth = now
	return tx.Model(&models.User{}).Where("id = ?", u.ID).
		Updates(map[string]any{
			"ai_credits":       u.AICredits,
			"ai_credits_month": u.AICreditsMonth,
		}).Error
}

// Refund 在生成失败时把额度还回去。今日次数不退,理由见 UsedToday。
//
// 收整条记录而不是 (userID, credits):退到哪本账、退哪一笔,读的必须是扣费
// 那一刻定格在记录上的值。这个函数刻意不接受 *models.User —— 拿不到用户,
// 就不可能不小心去看"他现在还绑着 pic.bi 吗"。那一眼就是漏洞:绑定 → 提交 →
// 立刻解绑 → 让生成失败,pic.bi 的真钱会被退成本地的免费次数,而解绑接口
// 一个 API token 就能调。
func Refund(db *gorm.DB, gen *models.AIGeneration) error {
	if gen == nil {
		return nil
	}
	if gen.RefundedAt != nil {
		return nil // 已经退过了
	}
	var err error
	if gen.Ledger == models.AIGenLedgerPicbi {
		err = refundRemote(gen)
	} else {
		err = refundLocal(db, gen.UserID, gen.Credits)
	}
	if err != nil {
		return err
	}
	// 盖章。没盖上不当成失败:钱已经还回去了,盖章失败最多让对账器再退一次,
	// 而退款的幂等键是确定的,重复的那次在 pic.bi 那边不会真的执行。反过来
	// 把它当失败上报,才会让调用方以为钱没还。
	now := time.Now()
	if db != nil {
		if e := db.Model(&models.AIGeneration{}).Where("id = ?", gen.ID).
			UpdateColumn("refunded_at", &now).Error; e != nil {
			log.Printf("ai: 退款已完成但落章失败 gen=%s ledger=%s: %v", gen.ID, gen.Ledger, e)
		}
	}
	gen.RefundedAt = &now
	return nil
}

// refundRemote 按定格下来的流水号原路退回 pic.bi。
func refundRemote(gen *models.AIGeneration) error {
	if remote == nil || !remote.Enabled() {
		// 这个部署把 pic.bi 关掉了,但库里还有它的历史记录。报错让调用方
		// 留痕,而不是悄悄按本地退——那是拿免费次数去顶真钱。
		return ErrRemoteUnavailable
	}
	opID := gen.SpendOpID
	if opID == "" {
		// 扣费那一步结果未知(超时,或者进程死在发出请求和存下流水号之间)。
		// 用同一个幂等键重放一次 spend:pic.bi 见过这个键就原样返回上次的
		// 流水号,于是我们拿回了退款的抓手;没见过就是新扣一笔、随即被下面
		// 退掉,净额为零。
		//
		// 两种结果都好过"当作没扣过":后者在真的扣过时,是用户充的钱凭空
		// 消失,而且没有任何记录指得回去。
		recovered, err := recoverSpendOp(gen)
		if err != nil {
			return err
		}
		if recovered == "" {
			return nil
		}
		log.Printf("picbi: 找回丢失的扣费流水 gen=%s op=%s", gen.ID, recovered)
		opID = recovered
	}
	ctx, cancel := remoteCtx()
	defer cancel()
	if err := remote.Refund(ctx, opID, refundKey(gen.ID)); err != nil {
		return mapRemoteErr(err)
	}
	return nil
}

// recoverSpendOp 用同一个幂等键重放 spend,把丢掉的流水号问回来。
//
// 返回 ("", nil) 表示"确实没有这笔钱"(比如对端回余额不足——既然现在都扣不
// 动,当初那次自然也没扣成)。返回错误表示"还是问不出来",调用方要留痕。
func recoverSpendOp(gen *models.AIGeneration) (string, error) {
	ctx, cancel := remoteCtx()
	defer cancel()
	opID, _, err := remote.Spend(ctx, gen.PicbiUserID, gen.Model, gen.Resolution,
		spendKey(gen.ID), Describe(gen.Prompt))
	if err != nil {
		if definitelyNotCharged(err) {
			return "", nil
		}
		return "", mapRemoteErr(err)
	}
	return opID, nil
}

// definitelyNotCharged 区分"对端读懂了并拒绝"与"我们不知道那边发生了什么"。
//
// 只有前者能推出"钱没动"。所以未知种类一律落在 false 一侧:在钱的问题上,
// 不确定要按"可能扣了"处理,多退一次的代价远小于少退一次。
func definitelyNotCharged(err error) bool {
	var k remoteKinder
	if !errors.As(err, &k) {
		return false
	}
	switch k.RemoteKind() {
	case "no_credits", "forbidden", "invalid":
		return true
	}
	return false
}

// refundLocal 是包级变量而不是普通函数:本地这条路要写数据库,而分流本身
// 该有测试。用一个可替换的函数值,测试才能在不起数据库的情况下断言
// "picbi 的记录没有走到这里来"。
var refundLocal = func(db *gorm.DB, userID uuid.UUID, credits int) error {
	if credits <= 0 {
		return nil
	}
	return db.Model(&models.User{}).Where("id = ?", userID).
		UpdateColumn("ai_credits", gorm.Expr("ai_credits + ?", credits)).Error
}

// GrantCheckin 是签到附带的赠送:随机 [min, max] 次。
//
// 永远只写本地账本,不分流。签到是"送",而 pic.bi 的分是用户充值的真钱、
// 还能在 pic.bi 站内消费——往那边送就是免费铸币。
//
// 返回实际送出的次数,好让签到接口能如实告诉用户拿到了什么。上限按当月额度
// 的两倍封顶——签到是锦上添花,不该让人靠天天签到把月配额顶到十倍。
func GrantCheckin(db *gorm.DB, u *models.User, g models.UserGroup) (int, error) {
	if g.AICheckinMax <= 0 {
		return 0, nil
	}
	if err := EnsureMonthly(db, u, g); err != nil {
		return 0, err
	}
	lo, hi := g.AICheckinMin, g.AICheckinMax
	if lo < 0 {
		lo = 0
	}
	if hi < lo {
		hi = lo
	}
	want := lo
	if hi > lo {
		want = lo + rand.IntN(hi-lo+1)
	}
	if want == 0 {
		return 0, nil
	}
	cap := g.AIMonthly * 2
	if cap > 0 && u.AICredits+want > cap {
		want = cap - u.AICredits
	}
	if want <= 0 {
		return 0, nil
	}
	if err := db.Model(&models.User{}).Where("id = ?", u.ID).
		UpdateColumn("ai_credits", gorm.Expr("ai_credits + ?", want)).Error; err != nil {
		return 0, err
	}
	u.AICredits += want
	return want, nil
}

// Describe 供日志与账本文案使用。
func Describe(prompt string) string {
	return fmt.Sprintf("AI 生成：%s", truncate(prompt, 60))
}
