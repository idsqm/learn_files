package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/andruho/files/internal/domain"
	"github.com/andruho/files/internal/repository"
	"github.com/andruho/files/internal/storage"
)

type FileService interface {
	Create(ctx context.Context, ownerUserID string, in domain.CreateFileInput) (*domain.CreateFileResult, error)
	Complete(ctx context.Context, id int, ownerUserID string) (*domain.File, error)
	GetByID(ctx context.Context, id int) (*domain.File, error)
	GetDownloadURL(ctx context.Context, id int) (string, error)
	Delete(ctx context.Context, id int, ownerUserID string) error
	ListByOwner(ctx context.Context, ownerUserID string, page, perPage int) ([]domain.File, int, error)
	CleanupStalePending(ctx context.Context) (int, error)
}

type fileService struct {
	files          repository.FileRepository
	store          storage.Storage
	presignedTTL   time.Duration
	pendingFileTTL time.Duration
	log            *slog.Logger
}

func NewFileService(files repository.FileRepository, store storage.Storage, presignedTTL, pendingFileTTL time.Duration, log *slog.Logger) FileService {
	return &fileService{
		files:          files,
		store:          store,
		presignedTTL:   presignedTTL,
		pendingFileTTL: pendingFileTTL,
		log:            log,
	}
}

func (s *fileService) Create(ctx context.Context, ownerUserID string, in domain.CreateFileInput) (*domain.CreateFileResult, error) {
	ve := domain.NewValidationErrors()
	if in.Filename == "" {
		ve.Add("filename", "required")
	}
	if in.MimeType == "" {
		ve.Add("mime_type", "required")
	}
	if in.SizeBytes <= 0 {
		ve.Add("size_bytes", "must be positive")
	}
	if !in.Kind.Valid() {
		ve.Add("kind", "must be one of: video, document, image")
	}
	if ve.HasErrors() {
		return nil, ve
	}

	if !domain.MimeAllowed(in.Kind, in.MimeType) {
		return nil, domain.ErrInvalidFileType
	}
	if maxSize := domain.MaxSizeForKind(in.Kind); in.SizeBytes > maxSize {
		return nil, domain.ErrFileTooLarge
	}

	storageKey := fmt.Sprintf("%s/%s/%s", in.Kind, uuid.NewString(), in.Filename)

	f := &domain.File{
		OwnerUserID: ownerUserID,
		Filename:    in.Filename,
		MimeType:    in.MimeType,
		Kind:        in.Kind,
		SizeBytes:   in.SizeBytes,
		StorageKey:  storageKey,
		Status:      domain.StatusPending,
	}

	id, err := s.files.Create(ctx, f)
	if err != nil {
		return nil, err
	}

	uploadURL, expiresAt, err := s.store.GeneratePutURL(ctx, storageKey, in.MimeType, s.presignedTTL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrStorageError, err)
	}

	return &domain.CreateFileResult{
		ID:        id,
		UploadURL: uploadURL,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *fileService) Complete(ctx context.Context, id int, ownerUserID string) (*domain.File, error) {
	f, err := s.files.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, domain.ErrFileNotFound
	}
	if f.OwnerUserID != ownerUserID {
		return nil, domain.ErrFileNotOwned
	}
	if f.Status == domain.StatusReady {
		return f, nil
	}

	info, err := s.store.HeadObject(ctx, f.StorageKey)
	if err != nil {
		if err == storage.ErrObjectNotFound {
			return nil, domain.ErrUploadNotConfirmed
		}
		return nil, fmt.Errorf("%w: %v", domain.ErrStorageError, err)
	}

	if info.SizeBytes > domain.MaxSizeForKind(f.Kind) {
		_ = s.store.DeleteObject(ctx, f.StorageKey)
		_ = s.files.MarkFailed(ctx, f.ID)
		return nil, domain.ErrFileTooLarge
	}
	if !domain.MimeAllowed(f.Kind, info.ContentType) {
		_ = s.store.DeleteObject(ctx, f.StorageKey)
		_ = s.files.MarkFailed(ctx, f.ID)
		return nil, domain.ErrInvalidFileType
	}

	if err := s.files.MarkReady(ctx, f.ID, info.SizeBytes); err != nil {
		return nil, err
	}

	f.Status = domain.StatusReady
	f.SizeBytes = info.SizeBytes
	now := time.Now()
	f.ReadyAt = &now
	return f, nil
}

func (s *fileService) GetByID(ctx context.Context, id int) (*domain.File, error) {
	f, err := s.files.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, domain.ErrFileNotFound
	}
	return f, nil
}

func (s *fileService) GetDownloadURL(ctx context.Context, id int) (string, error) {
	f, err := s.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if f.Status != domain.StatusReady {
		return "", domain.ErrFileNotReady
	}

	url, err := s.store.GenerateGetURL(ctx, f.StorageKey, s.presignedTTL)
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrStorageError, err)
	}
	return url, nil
}

func (s *fileService) Delete(ctx context.Context, id int, ownerUserID string) error {
	f, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if f.OwnerUserID != ownerUserID {
		return domain.ErrFileNotOwned
	}

	if err := s.store.DeleteObject(ctx, f.StorageKey); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrStorageError, err)
	}
	return s.files.Delete(ctx, f.ID)
}

func (s *fileService) ListByOwner(ctx context.Context, ownerUserID string, page, perPage int) ([]domain.File, int, error) {
	return s.files.ListByOwner(ctx, ownerUserID, page, perPage)
}

// CleanupStalePending deletes pending file records (and their orphaned storage objects,
// if any partial upload happened) that were never confirmed via /complete within
// pendingFileTTL. Returns the number of records removed.
func (s *fileService) CleanupStalePending(ctx context.Context) (int, error) {
	stale, err := s.files.ListStalePending(ctx, time.Now().Add(-s.pendingFileTTL))
	if err != nil {
		return 0, err
	}

	for _, f := range stale {
		if err := s.store.DeleteObject(ctx, f.StorageKey); err != nil {
			s.log.Warn("cleanup: failed to delete storage object", "file_id", f.ID, "storage_key", f.StorageKey, "error", err)
		}
		if err := s.files.Delete(ctx, f.ID); err != nil {
			s.log.Warn("cleanup: failed to delete file record", "file_id", f.ID, "error", err)
		}
	}

	return len(stale), nil
}

// RunCleanupLoop runs CleanupStalePending on a ticker until ctx is cancelled.
func RunCleanupLoop(ctx context.Context, svc FileService, interval time.Duration, log *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := svc.CleanupStalePending(ctx)
			if err != nil {
				log.Error("pending files cleanup failed", "error", err)
				continue
			}
			if n > 0 {
				log.Info("pending files cleanup", "deleted", n)
			}
		}
	}
}
