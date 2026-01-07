package dispatch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Cache interface {
	WriteSuccess(ctx context.Context, marker string, data []byte) error
	WriteFailure(ctx context.Context, marker string, data []byte) error
	SuccessModTime(ctx context.Context, marker string) (time.Time, error)
	FailureModTime(ctx context.Context, marker string) (time.Time, error)
	ReadSuccess(ctx context.Context, marker string) ([]byte, error)
	ReadFailure(ctx context.Context, marker string) ([]byte, error)
}

var ErrNotFound = errors.New("not found")

type fileCache struct {
	root string
}

func NewFileCache(root string) *fileCache {
	result := &fileCache{root: root}
	Must0(os.MkdirAll(filepath.Join(root, "success"), 0700))
	Must0(os.MkdirAll(filepath.Join(root, "failure"), 0700))
	return result
}

func (f *fileCache) successPath(marker string) string {
	return filepath.Join(f.root, "success", marker)
}

func (f *fileCache) failurePath(marker string) string {
	return filepath.Join(f.root, "failure", marker)
}

func (f *fileCache) WriteSuccess(ctx context.Context, marker string, data []byte) error {
	return os.WriteFile(f.successPath(marker), data, 0644)
}

func (f *fileCache) WriteFailure(ctx context.Context, marker string, data []byte) error {
	return os.WriteFile(f.failurePath(marker), data, 0644)
}

func (f *fileCache) SuccessModTime(ctx context.Context, marker string) (time.Time, error) {
	stat, err := os.Stat(f.successPath(marker))
	if err == nil {
		return stat.ModTime(), nil
	}
	return time.Time{}, ErrNotFound
}

func (f *fileCache) FailureModTime(ctx context.Context, marker string) (time.Time, error) {
	stat, err := os.Stat(f.failurePath(marker))
	if err == nil {
		return stat.ModTime(), nil
	}
	return time.Time{}, ErrNotFound
}

func (f *fileCache) ReadSuccess(ctx context.Context, marker string) ([]byte, error) {
	return os.ReadFile(f.successPath(marker))
}

func (f *fileCache) ReadFailure(ctx context.Context, marker string) ([]byte, error) {
	return os.ReadFile(f.failurePath(marker))
}
