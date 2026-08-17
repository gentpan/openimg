package models

import (
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

// IsTerminal 表示这条记录不会再变了。轮询与恢复都据此决定要不要继续跟进。
func (s AIGenStatus) IsTerminal() bool {
	return s == AIGenCompleted || s == AIGenFailed
}
