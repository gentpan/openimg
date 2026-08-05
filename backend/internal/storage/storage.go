// Package storage abstracts the S3-compatible buckets images live in. Unlike
// the single global backend this replaced, there are now many at once: the
// platform MinIO pool plus any number of user-owned R2/S3 buckets. Callers
// resolve a Backend from a models.StorageProfile through the Registry.
//
// Object keys are write-once and never reused (see models.ObjectKeyFor),
// which is what lets every response carry a one-year immutable cache header.
package storage

import (
	"context"
	"io"
	"strings"
)

// CacheControl is sent with every object. Safe because keys embed the content
// hash — the bytes behind a key can never change.
const CacheControl = "public, max-age=31536000, immutable"

// Backend is one bucket. Implementations must be safe for concurrent use.
type Backend interface {
	// Put stores body at key. size may be -1 when unknown, though S3
	// multipart efficiency suffers; the upload pipeline always knows it.
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	// Stat reports the object's size and whether it exists. A missing object
	// is (0, false, nil) — not an error.
	Stat(ctx context.Context, key string) (size int64, exists bool, err error)
	// Probe verifies credentials and write access end to end by round-tripping
	// a small test object. Used before persisting a user's bucket config.
	Probe(ctx context.Context) error
	// URL assembles the public CDN address of a key.
	URL(key string) string
	Kind() string
}

// ContentTypeFor maps a bare extension ("jpg") to its MIME type. Unknown
// extensions deliberately fall back to octet-stream so an unexpected format
// can never be served as something a browser will execute.
func ContentTypeFor(ext string) string {
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "avif":
		return "image/avif"
	case "gif":
		return "image/gif"
	case "bmp":
		return "image/bmp"
	case "tif", "tiff":
		return "image/tiff"
	case "heic":
		return "image/heic"
	default:
		return "application/octet-stream"
	}
}

// JoinURL builds base + "/" + key without doubling or dropping separators.
func JoinURL(base, key string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(key, "/")
}
