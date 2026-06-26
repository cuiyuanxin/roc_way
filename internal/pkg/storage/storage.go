// Package storage 提供统一文件存储接口与 local/oss 两种 driver。
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"

	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
)

// Storage 统一存储接口。
type Storage interface {
	Put(ctx context.Context, key string, r io.Reader) (string, error)
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	URL(key string) string
}

// New 根据 cfg.Driver 选择 driver。
func New(cfg config.StorageConfig) (Storage, error) {
	switch strings.ToLower(cfg.Driver) {
	case "", "local":
		return newLocal(cfg), nil
	case "oss":
		return newOSS(cfg)
	default:
		return nil, fmt.Errorf("storage: unknown driver %q", cfg.Driver)
	}
}

// ---------- local driver ----------

type localDriver struct {
	dir        string
	publicBase string
}

func newLocal(cfg config.StorageConfig) *localDriver {
	_ = os.MkdirAll(cfg.LocalDir, 0o755)
	return &localDriver{dir: cfg.LocalDir, publicBase: cfg.PublicBase}
}

// errInvalidKey 表示 key 不合法（含 ../ 路径遍历、空 key、绝对路径等）。
var errInvalidKey = errors.New("storage: invalid key")

// safePath 校验 key 不会逃出 baseDir 目录。
//
// 防御路径遍历攻击（OWASP Path Traversal）：
//   - 拒绝绝对路径（key 以 / 或盘符开头）；
//   - filepath.Clean 后再次确认结果仍在 baseDir 下；
//   - 拒绝空 key / 仅空白字符 key。
func safePath(baseDir, key string) (string, error) {
	if key == "" || strings.TrimSpace(key) == "" {
		return "", errInvalidKey
	}
	cleaned := filepath.Clean(filepath.FromSlash(key))
	if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, ".."+string(filepath.Separator)) {
		return "", errInvalidKey
	}
	full := filepath.Join(baseDir, cleaned)
	rel, err := filepath.Rel(baseDir, full)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", errInvalidKey
	}
	return full, nil
}

func (l *localDriver) Put(_ context.Context, key string, r io.Reader) (string, error) {
	full, err := safePath(l.dir, key)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(full)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return l.URL(key), nil
}

func (l *localDriver) Get(_ context.Context, key string) (io.ReadCloser, error) {
	full, err := safePath(l.dir, key)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

func (l *localDriver) Delete(_ context.Context, key string) error {
	full, err := safePath(l.dir, key)
	if err != nil {
		return err
	}
	return os.Remove(full)
}

func (l *localDriver) URL(key string) string {
	return strings.TrimRight(l.publicBase, "/") + "/" + strings.TrimLeft(key, "/")
}

// ---------- oss driver ----------

type ossDriver struct {
	bucket *oss.Bucket
	base   string
}

func newOSS(cfg config.StorageConfig) (*ossDriver, error) {
	cli, err := oss.New(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("storage: oss new: %w", err)
	}
	bucket, err := cli.Bucket(cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("storage: oss bucket: %w", err)
	}
	return &ossDriver{bucket: bucket, base: cfg.PublicBase}, nil
}

func (o *ossDriver) Put(ctx context.Context, key string, r io.Reader) (string, error) {
	if err := o.bucket.PutObject(key, r); err != nil {
		return "", err
	}
	return o.URL(key), nil
}

func (o *ossDriver) Get(_ context.Context, key string) (io.ReadCloser, error) {
	body, err := o.bucket.GetObject(key)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (o *ossDriver) Delete(_ context.Context, key string) error {
	return o.bucket.DeleteObject(key)
}

func (o *ossDriver) URL(key string) string {
	if o.base != "" {
		return strings.TrimRight(o.base, "/") + "/" + strings.TrimLeft(key, "/")
	}
	signed, _ := o.bucket.SignURL(key, oss.HTTPGet, 3600)
	return signed
}
