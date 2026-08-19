package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AIGeneration 是一次文生图请求的全过程记录。
//
// 单独一张表而不是塞进 Image:生成是异步的,提交之后要轮询上游、可能失败、
// 可能超时,这些状态与"一张已经存在的图"无关。成功之后才产出一条 Image,
// 两者用 ImageID 关联——从那一刻起它就是一张普通图片,去重、变体、外链、
// 短链全都照常。
type AIGeneration struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID uuid.UUID `gorm:"type:uuid;index;not null" json:"-"`

	// Kind 区分"凭空生成"与"拿已有的图去改"。两者的流程完全一样(扣费、
	// 递交、轮询、退款、入库),差别只在提交时多带几张源图,所以是同一张表
	// 的一个字段,不是第二张表。
	//
	// 存量记录这一列是空的,故意不写迁移去刷:空值按 generate 读(见
	// AfterFind),存量行本来也只可能是文生图。
	Kind string `gorm:"size:16;not null;default:'generate'" json:"kind"`
	// SourceIDs 是逗号分隔的源图 ID,空表示纯生成。存 CSV 而不是开一张关联
	// 表:它只被整体读写、从不单独查询,一张表换来的只有 join。
	SourceIDs string `gorm:"type:text" json:"source_ids"`

	Prompt string `gorm:"type:text;not null" json:"prompt"`
	Model  string `gorm:"size:64;not null" json:"model"`
	Size   string `gorm:"size:16;not null" json:"size"`
	// Resolution 是计费档位(1k/2k/4k),不是像素尺寸。
	Resolution string `gorm:"size:8;not null" json:"resolution"`

	// TaskID 是上游返回的任务号。轮询靠它,重启后恢复也靠它。
	TaskID  string      `gorm:"size:128;index" json:"-"`
	Status  AIGenStatus `gorm:"size:16;index;not null" json:"status"`
	Error   string      `gorm:"type:text" json:"error,omitempty"`
	ImageID *uuid.UUID  `gorm:"type:uuid;index" json:"image_id,omitempty"`

	// Credits 记录这次实际扣了几点。失败退还时按这个数退,而不是假定扣了 1
	// ——将来支持一次多张时这里就是 n。走 pic.bi 的记录里它是对端算出来并
	// 返回的价钱(1k/2k/4k = 1/2/4),不是本地写死的常数。
	Credits int `gorm:"not null;default:1" json:"credits"`

	// Hidden 是用户在列表里"删掉"这条记录后置上的位:界面不再显示,行仍在。
	//
	// 不能用 gorm.DeletedAt。GORM 的软删除会给这张表的每一条查询自动补上
	// `deleted_at IS NULL`,包括 UsedToday 里那个 Count——而那个 Count 就是
	// 日限本身。一旦被静默改写,用户删一条记录就等于把当天的配额退回来一次,
	// 删多少次就能白嫖多少次。用一个普通布尔列,过滤只发生在真正想过滤的那
	// 一处(列表接口),别处的查询看不见它、也就不会被它改变。
	//
	// 同理这里也不做真删:退款重试(retryFailedRefunds)要扫失败行,对账要认
	// 领孤儿,真删会把未结清的账一起删掉。
	Hidden bool `gorm:"not null;default:false;index" json:"-"`

	// 账本归属。这三列在扣费那一刻写死,之后只读不改。
	//
	// 为什么不在退款时现查用户绑没绑 pic.bi:那条路上有一个能把真钱换成免费
	// 额度的洞。绑定 → 提交生成(扣了 pic.bi 的真钱)→ 立刻解绑 → 让生成失败,
	// 退款如果去看"当前绑定状态",会认为这是一笔本地扣费,把钱退进本地的
	// 免费次数里。而解绑接口在机器组,一个 API token 就能调。
	//
	// 所以退款只认这里:Ledger 决定退到哪边,SpendOpID 决定退哪一笔,金额由
	// pic.bi 从原流水里读。
	//
	// 存量记录这一列是空的,按 local 读(见 AfterFind)——存量本来也全是本地
	// 账本。注意按账本筛选时要写 ledger <> 'picbi' 而不是 ledger = 'local',
	// 空串才落得进去。
	Ledger      string `gorm:"size:16;not null;default:'local'" json:"ledger"`
	PicbiUserID string `gorm:"size:64;index" json:"-"`
	// SpendOpID 是 pic.bi 那笔扣费流水的号。空表示扣费那一步没走完——可能
	// 压根没扣,也可能扣成了但回执没回来,两者在这里分不出来,由退款那一侧
	// 用同一个幂等键重放去问。
	SpendOpID string `gorm:"size:64;index" json:"-"`
	// RefundedAt 是"这条失败记录的钱已经还回去了"的凭据。
	//
	// 它存在是因为远程退款会真的失败(网络抖一下就够了),而记录一旦落成
	// failed 就是终态、再没有人回来看它。只打一行日志的话,丢的是用户充值的
	// 真钱。有了这一列,对账器才能把没退成的挑出来重试——重试安全,因为
	// 退款的幂等键是由记录 ID 算出来的,同一笔退两次在 pic.bi 那边是一次。
	RefundedAt *time.Time `gorm:"index" json:"-"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"-"`
	DoneAt    *time.Time `json:"done_at,omitempty"`
}

// AIGeneration.Kind 的取值。
const (
	AIGenKindGenerate = "generate"
	AIGenKindEdit     = "edit"
)

// AIGeneration.Ledger 的取值:这次的额度记在谁家账上。
const (
	AIGenLedgerLocal = "local"
	AIGenLedgerPicbi = "picbi"
)

type AIGenStatus string

const (
	// AIGenCharging 是"额度已扣、还没递交给上游"的一瞬。
	//
	// 它存在的理由是日限:UsedToday 数的是这张表的行,如果行要等递交成功
	// 才写,并发的请求就会全部读到同一个旧计数、全部放行。把行提到扣费
	// 那一刻落库,它才能同时当计数器用。
	//
	// 副产品是对账有了抓手:一条停在 charging 又没有 TaskID 的老记录,
	// 就是扣了费却没递交出去的孤儿。
	AIGenCharging  AIGenStatus = "charging"
	AIGenPending   AIGenStatus = "pending"
	AIGenRunning   AIGenStatus = "running"
	AIGenCompleted AIGenStatus = "completed"
	AIGenFailed    AIGenStatus = "failed"
)

func (g *AIGeneration) BeforeCreate(tx *gorm.DB) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	return nil
}

// AfterFind 把存量记录的空 Kind 归一成 generate。
//
// 在读的时候补,而不是写一条 UPDATE 去刷存量:客户端拿到的 kind 一律是有效
// 值,而数据库里那些历史行不必被动过——一次性迁移脚本会在每个自建部署上再
// 跑一遍,而这行代码到处都成立。
func (g *AIGeneration) AfterFind(tx *gorm.DB) error {
	if g.Kind == "" {
		g.Kind = AIGenKindGenerate
	}
	if g.Ledger == "" {
		g.Ledger = AIGenLedgerLocal
	}
	return nil
}

// SourceIDList 把 CSV 拆回 ID 列表,跳过存不下的脏值。
func (g *AIGeneration) SourceIDList() []uuid.UUID {
	if g.SourceIDs == "" {
		return nil
	}
	parts := strings.Split(g.SourceIDs, ",")
	out := make([]uuid.UUID, 0, len(parts))
	for _, p := range parts {
		id, err := uuid.Parse(strings.TrimSpace(p))
		if err != nil {
			continue
		}
		out = append(out, id)
	}
	return out
}

// JoinSourceIDs 是 SourceIDList 的反向:拼成入库用的 CSV。
func JoinSourceIDs(ids []uuid.UUID) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	return strings.Join(parts, ",")
}

// IsTerminal 表示这条记录不会再变了。轮询与恢复都据此决定要不要继续跟进。
func (s AIGenStatus) IsTerminal() bool {
	return s == AIGenCompleted || s == AIGenFailed
}
