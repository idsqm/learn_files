package storage

import (
	"context"
	"errors"
	"time"
)

// ErrObjectNotFound is returned by HeadObject when the key does not exist in the bucket.
var ErrObjectNotFound = errors.New("storage: object not found")

type ObjectInfo struct {
	SizeBytes   int64
	ContentType string
}

// Storage abstracts the S3-compatible object store. The same interface and implementation
// work against MinIO (dev/self-hosted) and AWS S3 / Cloudflare R2 (prod) — only Endpoint and
// UsePathStyle change.
type Storage interface {
	// GeneratePutURL returns a presigned PUT URL for key, valid for ttl. The Content-Type header
	// is bound into the signature, so an upload with a different Content-Type will be rejected by
	// the bucket with a signature mismatch.
	GeneratePutURL(ctx context.Context, key, contentType string, ttl time.Duration) (url string, expiresAt time.Time, err error)

	// GenerateGetURL returns a presigned GET URL for key, valid for ttl.
	GenerateGetURL(ctx context.Context, key string, ttl time.Duration) (url string, err error)

	// HeadObject returns metadata for key, or ErrObjectNotFound if it doesn't exist.
	HeadObject(ctx context.Context, key string) (*ObjectInfo, error)

	// DeleteObject removes key. Deleting a non-existent key is not an error (S3 semantics).
	DeleteObject(ctx context.Context, key string) error
}
