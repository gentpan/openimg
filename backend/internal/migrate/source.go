// Package migrate 把别处的图搬进来。
//
// 只做单向导入,不做"传到别家"。理由是导入进来的图从入库那一刻起就是一张
// 普通的 openimg 图片——去重、变体、外链、短链、备份全都有;而反过来把图
// 传去别家,图库、配额、去重全都失效,用户得到的是一个残缺的第二套系统。
//
// 每种来源只需回答一个问题:"下一批图的地址是什么"。抓取、入库、配额、
// 去重都由调用方走既有的上传流水线,来源实现不碰这些。
package migrate

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Item 是一张待导入的图。
type Item struct {
	// URL 是可直接下载的地址。
	URL string
	// Name 用作图库里的展示名;留空则从 URL 末段取。
	Name string
	// ExternalID 是这张图在原站的标识,用于跳过已导入的。可以为空。
	ExternalID string
}

// Page 是一次列举的结果。Cursor 为空表示没有下一页了。
type Page struct {
	Items  []Item
	Cursor string
}

// Source 是一个可枚举的图片来源。
//
// 分页用游标而不是页码:兰空按页、S3 按 continuation token、SM.MS 按页,
// 统一成不透明的字符串,任务表里存着它就能断点续传。
type Source interface {
	// Kind 是这种来源的标识,与数据库里存的一致。
	Kind() string
	// List 取一批。cursor 为空表示从头开始。
	List(ctx context.Context, cursor string, limit int) (Page, error)
}

// Config 是建一个来源需要的全部参数。不同来源用到的字段不同,没用到的留空。
type Config struct {
	Kind string
	// Endpoint 是站点地址(兰空/自建)或 S3 的 endpoint。
	Endpoint string
	// Token 是 API 令牌;S3 用 AccessKey/SecretKey。
	Token     string
	AccessKey string
	SecretKey string
	Region    string
	Bucket    string
	Prefix    string
	// URLs 供"粘贴一批地址"这种来源使用。
	URLs []string
}

var ErrUnknownKind = errors.New("migrate: 不认识的来源类型")

// New 按 Kind 造一个来源。
//
// 每个实现自己校验参数——缺什么、格式不对,在这里就说清楚,而不是等到
// 导入跑起来才一条条失败。
func New(cfg Config) (Source, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Kind)) {
	case KindSMMS:
		return newSMMS(cfg)
	case KindLsky:
		return newLsky(cfg)
	case KindS3:
		return newS3(cfg)
	case KindURLList:
		return newURLList(cfg)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownKind, cfg.Kind)
	}
}

const (
	KindSMMS    = "smms"
	KindLsky    = "lsky"
	KindS3      = "s3"
	KindURLList = "urls"
)

// Kinds 是界面上可选的来源清单。
func Kinds() []string { return []string{KindSMMS, KindLsky, KindS3, KindURLList} }
