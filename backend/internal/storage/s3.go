package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
)

// S3 talks to any S3-compatible bucket: our MinIO pool, Cloudflare R2,
// Backblaze B2, AWS proper. The differences that matter are path-style
// addressing (MinIO wants it, R2 doesn't) and ACL support (R2 rejects them).
type S3 struct {
	client        *s3.Client
	bucket        string
	prefix        string
	publicURLBase string
	kind          string
}

type S3Config struct {
	Endpoint      string // "https://minio.internal:9000" or R2's account endpoint
	Region        string // "auto" for R2/MinIO
	Bucket        string
	AccessKey     string
	SecretKey     string
	PublicURLBase string // CDN origin, e.g. "https://files.openimgcdn.com"
	ThumbURLBase  string
	KeyPrefix     string
	UsePathStyle  bool // true for MinIO, false for R2
	Kind          string
}

func NewS3(ctx context.Context, c S3Config) (*S3, error) {
	if c.Bucket == "" {
		return nil, fmt.Errorf("s3: bucket required")
	}
	if c.PublicURLBase == "" {
		return nil, fmt.Errorf("s3: public_url_base required")
	}
	region := c.Region
	if region == "" {
		region = "auto"
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("s3 config: %w", err)
	}
	cli := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if c.Endpoint != "" {
			o.BaseEndpoint = aws.String(c.Endpoint)
		}
		o.UsePathStyle = c.UsePathStyle
	})
	kind := c.Kind
	if kind == "" {
		kind = "s3"
	}
	return &S3{
		client:        cli,
		bucket:        c.Bucket,
		prefix:        strings.TrimLeft(c.KeyPrefix, "/"),
		publicURLBase: strings.TrimRight(c.PublicURLBase, "/"),
		kind:          kind,
	}, nil
}

func (s *S3) Kind() string { return s.kind }

// full applies the configured prefix. Stored keys in the DB are prefix-free so
// the prefix can be changed without rewriting rows.
func (s *S3) full(key string) string { return s.prefix + strings.TrimLeft(key, "/") }

func (s *S3) URL(key string) string { return JoinURL(s.publicURLBase, s.full(key)) }

func (s *S3) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	in := &s3.PutObjectInput{
		Bucket:       aws.String(s.bucket),
		Key:          aws.String(s.full(key)),
		Body:         body,
		ContentType:  aws.String(contentType),
		CacheControl: aws.String(CacheControl),
		// Stored on the object so the CDN echoes it. Original-mode uploads keep
		// their bytes verbatim, so this is what stops a polyglot file from
		// being sniffed as HTML.
		ContentDisposition: aws.String("inline"),
		Metadata:           map[string]string{"x-content-type-options": "nosniff"},
		ACL:                types.ObjectCannedACLPublicRead,
	}
	if size >= 0 {
		in.ContentLength = aws.Int64(size)
	}
	_, err := s.client.PutObject(ctx, in)
	if err == nil {
		return nil
	}
	// R2 and some MinIO policies reject canned ACLs outright. Retry without —
	// public read is granted by bucket policy there instead.
	if !mentionsACL(err) {
		return err
	}
	rewound, rerr := rewind(body)
	if rerr != nil {
		return fmt.Errorf("s3 put (acl retry): %w", rerr)
	}
	in.ACL = ""
	in.Body = rewound
	_, err = s.client.PutObject(ctx, in)
	return err
}

// rewind returns a reader positioned back at the start. Only seekable bodies
// and in-memory buffers can be retried; anything else must be re-read by the
// caller, which the upload pipeline handles by buffering variants.
func rewind(body io.Reader) (io.Reader, error) {
	if s, ok := body.(io.Seeker); ok {
		if _, err := s.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		return body, nil
	}
	return nil, errors.New("body is not seekable, cannot retry")
}

func mentionsACL(err error) bool {
	e := strings.ToLower(err.Error())
	return strings.Contains(e, "acl") || strings.Contains(e, "not implemented")
}

func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.full(key)),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (s *S3) Stat(ctx context.Context, key string) (int64, bool, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.full(key)),
	})
	if err != nil {
		var nf *types.NotFound
		var nsk *types.NoSuchKey
		if errors.As(err, &nf) || errors.As(err, &nsk) {
			return 0, false, nil
		}
		// Some implementations answer HEAD on a missing key with a bare 404
		// that doesn't map to a typed error.
		if strings.Contains(err.Error(), "StatusCode: 404") {
			return 0, false, nil
		}
		return 0, false, err
	}
	if out.ContentLength == nil {
		return 0, true, nil
	}
	return *out.ContentLength, true, nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.full(key)),
	})
	return err
}

// Probe round-trips a throwaway object. HeadBucket alone isn't enough: a
// read-only token passes it and then fails on the first real upload, which is
// exactly the misconfiguration we want to catch while the user is still
// looking at the form.
func (s *S3) Probe(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	if _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)}); err != nil {
		return fmt.Errorf("无法访问 bucket %q：%w", s.bucket, err)
	}

	key := ".openimg-probe/" + uuid.NewString()
	payload := []byte("openimg storage probe")
	if err := s.Put(ctx, key, bytes.NewReader(payload), int64(len(payload)), "text/plain"); err != nil {
		return fmt.Errorf("写入测试对象失败（凭据可能只有只读权限）：%w", err)
	}
	// Clean up even if the read-back fails — never leave probe litter behind.
	defer func() { _ = s.Delete(context.WithoutCancel(ctx), key) }()

	rc, err := s.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("读回测试对象失败：%w", err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("读取测试对象内容失败：%w", err)
	}
	if !bytes.Equal(got, payload) {
		return fmt.Errorf("测试对象内容不一致，bucket 可能被其他服务占用")
	}
	return nil
}
