package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Lister 列举一个桶里已有的对象。
//
// 它刻意不在 Backend 接口里。Backend 描述的是"我们往哪写"——Put/Get/Delete
// 就够了,加一个 List 会逼着每个后端都实现一遍它们平时用不上的能力。而列举
// 只在迁移时用到一次:把别人桶里的存量搬进来。两件事,两个类型。
type Lister interface {
	// List 取一批对象。cursor 为空表示从头开始,返回的 cursor 为空表示到头了。
	List(ctx context.Context, prefix, cursor string, limit int) ([]Object, string, error)
}

// Object 是桶里的一个对象。URL 是可直接下载的地址。
type Object struct {
	Key  string
	Size int64
	URL  string
}

type ListerConfig struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
}

// presignTTL 是列举时签出的下载地址的有效期。
//
// 迁移是边列边下的,一批地址拿到手上几分钟内就用掉了。给一小时是为了容忍
// 队列积压——但不给更长:这些地址一旦泄露就是对用户私有桶的直接读权限。
const presignTTL = time.Hour

type s3Lister struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

// NewLister 连上一个用户给的 S3 兼容桶。
//
// 与 BYOS 走同一道 endpoint 闸:用户能填地址的地方,就是攻击者能让本服务替他
// 探测内网的地方。
func NewLister(c ListerConfig) (Lister, error) {
	if strings.TrimSpace(c.Bucket) == "" {
		return nil, errors.New("storage: 需要桶名")
	}
	if c.Endpoint != "" {
		if err := ValidateUserEndpoint(c.Endpoint); err != nil {
			return nil, err
		}
	}
	region := c.Region
	if region == "" {
		region = "auto"
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	cli := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if c.Endpoint != "" {
			o.BaseEndpoint = aws.String(strings.TrimRight(c.Endpoint, "/"))
		}
		// 路径式寻址:用户填的往往是 MinIO 或自建网关,它们不支持
		// bucket.host 这种虚拟主机式的域名。R2/AWS 两种都认。
		o.UsePathStyle = true
	})
	return &s3Lister{
		client:  cli,
		presign: s3.NewPresignClient(cli),
		bucket:  strings.TrimSpace(c.Bucket),
	}, nil
}

func (l *s3Lister) List(ctx context.Context, prefix, cursor string, limit int) ([]Object, string, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	in := &s3.ListObjectsV2Input{
		Bucket:  aws.String(l.bucket),
		MaxKeys: aws.Int32(int32(limit)),
	}
	if p := strings.TrimLeft(prefix, "/"); p != "" {
		in.Prefix = aws.String(p)
	}
	if cursor != "" {
		in.ContinuationToken = aws.String(cursor)
	}
	out, err := l.client.ListObjectsV2(ctx, in)
	if err != nil {
		return nil, "", fmt.Errorf("storage: 列举失败: %w", err)
	}

	objs := make([]Object, 0, len(out.Contents))
	for _, o := range out.Contents {
		key := aws.ToString(o.Key)
		if key == "" || strings.HasSuffix(key, "/") {
			continue
		}
		// 预签而不是拼公开地址:迁移源多半是私有桶,拼出来的公开地址会
		// 403。预签用的就是用户自己给的那把钥匙,能列举就能签。
		req, err := l.presign.PresignGetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(l.bucket),
			Key:    aws.String(key),
		}, s3.WithPresignExpires(presignTTL))
		if err != nil {
			return nil, "", fmt.Errorf("storage: 签发下载地址失败: %w", err)
		}
		objs = append(objs, Object{Key: key, Size: aws.ToInt64(o.Size), URL: req.URL})
	}

	next := ""
	if aws.ToBool(out.IsTruncated) {
		next = aws.ToString(out.NextContinuationToken)
	}
	return objs, next, nil
}
