package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Local keeps objects on the filesystem and serves them through Gin's static
// handler. It exists so the stack runs end to end without MinIO during
// development; production always resolves to S3.
type Local struct {
	root          string
	publicURLBase string
}

func NewLocal(root, publicURLBase string) *Local {
	return &Local{
		root:          root,
		publicURLBase: strings.TrimRight(publicURLBase, "/"),
	}
}

func (l *Local) Kind() string { return "local" }

// path resolves a key under root, refusing anything that escapes it. Keys are
// generated internally, but this is the one place a traversal would land.
func (l *Local) path(key string) (string, error) {
	clean := filepath.Clean("/" + strings.TrimLeft(key, "/"))
	full := filepath.Join(l.root, clean)
	if !strings.HasPrefix(full, filepath.Clean(l.root)+string(os.PathSeparator)) {
		return "", fmt.Errorf("local: key %q escapes storage root", key)
	}
	return full, nil
}

func (l *Local) URL(key string) string {
	return JoinURL(l.publicURLBase, "storage/"+strings.TrimLeft(key, "/"))
}

func (l *Local) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	full, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	// Write to a temp file in the same directory, then rename: a reader can
	// never observe a half-written object.
	tmp, err := os.CreateTemp(filepath.Dir(full), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if _, err := io.Copy(tmp, body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, full)
}

func (l *Local) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	full, err := l.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

func (l *Local) Stat(ctx context.Context, key string) (int64, bool, error) {
	full, err := l.path(key)
	if err != nil {
		return 0, false, err
	}
	fi, err := os.Stat(full)
	if os.IsNotExist(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return fi.Size(), true, nil
}

func (l *Local) Delete(ctx context.Context, key string) error {
	full, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (l *Local) Probe(ctx context.Context) error {
	key := ".openimg-probe/local"
	payload := []byte("openimg storage probe")
	if err := l.Put(ctx, key, bytes.NewReader(payload), int64(len(payload)), "text/plain"); err != nil {
		return fmt.Errorf("写入本地存储失败：%w", err)
	}
	defer func() { _ = l.Delete(ctx, key) }()
	rc, err := l.Get(ctx, key)
	if err != nil {
		return err
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, payload) {
		return fmt.Errorf("本地存储读写内容不一致")
	}
	return nil
}
