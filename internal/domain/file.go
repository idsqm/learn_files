package domain

import (
	"strings"
	"time"
)

type Kind string

const (
	KindVideo    Kind = "video"
	KindDocument Kind = "document"
	KindImage    Kind = "image"
)

func (k Kind) Valid() bool {
	switch k {
	case KindVideo, KindDocument, KindImage:
		return true
	}
	return false
}

type Status string

const (
	StatusPending Status = "pending"
	StatusReady   Status = "ready"
	StatusFailed  Status = "failed"
)

const (
	maxVideoSize    = 4 << 30   // 4 GB
	maxDocumentSize = 100 << 20 // 100 MB
	maxImageSize    = 10 << 20  // 10 MB
)

var allowedDocumentMimes = map[string]bool{
	"application/pdf": true,
	"application/zip": true,
}

// MaxSizeForKind returns the byte limit for the given kind, or 0 if the kind is unknown.
func MaxSizeForKind(k Kind) int64 {
	switch k {
	case KindVideo:
		return maxVideoSize
	case KindDocument:
		return maxDocumentSize
	case KindImage:
		return maxImageSize
	}
	return 0
}

// MimeAllowed reports whether mimeType is acceptable for the given kind.
func MimeAllowed(k Kind, mimeType string) bool {
	switch k {
	case KindVideo:
		return strings.HasPrefix(mimeType, "video/")
	case KindImage:
		return strings.HasPrefix(mimeType, "image/")
	case KindDocument:
		return allowedDocumentMimes[mimeType]
	}
	return false
}

type File struct {
	ID          int        `json:"id"`
	OwnerUserID string     `json:"owner_user_id"`
	Filename    string     `json:"filename"`
	MimeType    string     `json:"mime_type"`
	Kind        Kind       `json:"kind"`
	SizeBytes   int64      `json:"size_bytes"`
	StorageKey  string     `json:"-"`
	Status      Status     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ReadyAt     *time.Time `json:"ready_at,omitempty"`
}

type CreateFileInput struct {
	Filename  string
	MimeType  string
	SizeBytes int64
	Kind      Kind
}

type CreateFileResult struct {
	ID        int       `json:"id"`
	UploadURL string    `json:"upload_url"`
	ExpiresAt time.Time `json:"expires_at"`
}
