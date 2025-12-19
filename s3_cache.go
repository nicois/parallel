package parallel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/nicois/bigset"
)

type MTime struct {
	Path  string
	Mtime time.Time
}
type s3Cache struct {
	client *s3.Client
	bucket string
	prefix string
	mtimes *bigset.Bigset[MTime]
}

func NewS3Cache(ctx context.Context, uri string) (Cache, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg)

	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "s3" {
		return nil, fmt.Errorf("invalid scheme: %s", u.Scheme)
	}

	mtimes, err := bigset.Create[MTime](nil, bigset.WithKeyFunction(func(m *MTime) []byte {
		return []byte(m.Path)
	}))
	if err != nil {
		return nil, err
	}

	result := &s3Cache{client: client, bucket: u.Host, prefix: strings.TrimPrefix(u.Path, "/"), mtimes: mtimes}
	return result, result.loadMtimes(ctx)
}

func (f *s3Cache) loadMtimes(ctx context.Context) error {
	paginator := s3.NewListObjectsV2Paginator(f.client, &s3.ListObjectsV2Input{Bucket: &(f.bucket)})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		for _, obj := range page.Contents {
			if _, err := f.mtimes.Add(ctx, "default", MTime{Path: *(obj.Key), Mtime: *(obj.LastModified)}); err != nil {
				return err
			}
		}
	}
	logger.Debug("loaded mtime", slog.Int64("count", Must(f.mtimes.Cardinality(ctx, "default"))))
	return nil
}

func (f *s3Cache) successPath(marker string) string {
	return strings.TrimPrefix(filepath.Join(f.prefix, "success", marker), "/")
}

func (f *s3Cache) failurePath(marker string) string {
	return strings.TrimPrefix(filepath.Join(f.prefix, "failure", marker), "/")
}

func (f *s3Cache) WriteSuccess(ctx context.Context, marker string, data []byte) error {
	return f.put(ctx, f.successPath(marker), data)
}

func (f *s3Cache) WriteFailure(ctx context.Context, marker string, data []byte) error {
	return f.put(ctx, f.failurePath(marker), data)
}

func (f *s3Cache) put(ctx context.Context, path string, data []byte) error {
	_, err := f.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &(f.bucket), Key: &path, Body: bytes.NewReader(data)})
	return err
}

func (f *s3Cache) SuccessModTime(ctx context.Context, marker string) (time.Time, error) {
	return f.fetchMtime(ctx, f.successPath(marker))
}

func (f *s3Cache) fetchMtime(ctx context.Context, path string) (time.Time, error) {
	mtime, err := f.mtimes.RetrieveIfExists(ctx, "default", MTime{Path: path})
	if err != nil || mtime == nil {
		return time.Time{}, errors.New("ntime not available as the path does not exist")
	}
	return mtime.Mtime, nil
}

func (f *s3Cache) FailureModTime(ctx context.Context, marker string) (time.Time, error) {
	return f.fetchMtime(ctx, f.failurePath(marker))
}

func (f *s3Cache) ReadSuccess(ctx context.Context, marker string) ([]byte, error) {
	return f.read(ctx, f.successPath(marker))
}

func (f *s3Cache) read(ctx context.Context, key string) ([]byte, error) {
	output, err := f.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &(f.bucket), Key: &key})
	if err != nil {
		return nil, err
	}
	return readCloserToBytes(output.Body)
}

func (f *s3Cache) ReadFailure(ctx context.Context, marker string) ([]byte, error) {
	return f.read(ctx, f.failurePath(marker))
}

func readCloserToBytes(rc io.ReadCloser) ([]byte, error) {
	// Ensure the ReadCloser is closed to prevent resource leaks
	defer func() {
		_ = rc.Close()
	}()

	// Read the entire stream into a byte slice
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	return data, nil
}
