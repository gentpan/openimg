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
	TaskID string       `gorm:"size:128;index" json:"-"`
	Status AIGenStatus  `gorm:"size:16;index;not null" json:"status"`
	Error  string       `gorm:"type:text" json:"error,omitempty"`
	ImageID *uuid.UUID  `gorm:"type:uuid;index" json:"image_id,omitempty"`

	// Credits 记录这次实际扣了几点。失败退还时按这个数退,而不是假定扣了 1
	// ——将来支持一次多张时这里就是 n。
	Credits int `gorm:"not null;default:1" json:"credits"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"-"`
	DoneAt    *time.Time `json:"done_at,omitempty"`
}

// AIGeneration.Kind 的取值。
const (
	AIGenKindGenerate = "generate"
	AIGenKindEdit     = "edit"
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
