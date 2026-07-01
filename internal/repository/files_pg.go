package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andruho/files/internal/domain"
)

type FileRepository interface {
	Create(ctx context.Context, f *domain.File) (int, error)
	GetByID(ctx context.Context, id int) (*domain.File, error)
	MarkReady(ctx context.Context, id int, actualSizeBytes int64) error
	MarkFailed(ctx context.Context, id int) error
	Delete(ctx context.Context, id int) error
	ListByOwner(ctx context.Context, ownerUserID string, page, perPage int) ([]domain.File, int, error)
	ListStalePending(ctx context.Context, olderThan time.Time) ([]domain.File, error)
}

type fileRepo struct {
	pool *pgxpool.Pool
}

func NewFileRepository(pool *pgxpool.Pool) FileRepository {
	return &fileRepo{pool: pool}
}

func (r *fileRepo) Create(ctx context.Context, f *domain.File) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx, `
		INSERT INTO files (owner_user_id, filename, mime_type, kind, size_bytes, storage_key, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending')
		RETURNING id
	`, f.OwnerUserID, f.Filename, f.MimeType, f.Kind, f.SizeBytes, f.StorageKey).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *fileRepo) GetByID(ctx context.Context, id int) (*domain.File, error) {
	var f domain.File
	err := r.pool.QueryRow(ctx, `
		SELECT id, owner_user_id, filename, mime_type, kind, size_bytes, storage_key, status, created_at, ready_at
		FROM files WHERE id = $1
	`, id).Scan(&f.ID, &f.OwnerUserID, &f.Filename, &f.MimeType, &f.Kind, &f.SizeBytes, &f.StorageKey, &f.Status, &f.CreatedAt, &f.ReadyAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

// MarkReady transitions a file to ready, recording the actual size confirmed in storage
// (via HEAD) as the source of truth — it may differ from the size declared at creation.
func (r *fileRepo) MarkReady(ctx context.Context, id int, actualSizeBytes int64) error {
	_, err := r.pool.Exec(ctx,
		"UPDATE files SET status = 'ready', ready_at = NOW(), size_bytes = $2 WHERE id = $1", id, actualSizeBytes)
	return err
}

func (r *fileRepo) MarkFailed(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx,
		"UPDATE files SET status = 'failed' WHERE id = $1", id)
	return err
}

func (r *fileRepo) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, "DELETE FROM files WHERE id = $1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrFileNotFound
	}
	return nil
}

func (r *fileRepo) ListByOwner(ctx context.Context, ownerUserID string, page, perPage int) ([]domain.File, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM files WHERE owner_user_id = $1", ownerUserID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	rows, err := r.pool.Query(ctx, `
		SELECT id, owner_user_id, filename, mime_type, kind, size_bytes, storage_key, status, created_at, ready_at
		FROM files
		WHERE owner_user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, ownerUserID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var files []domain.File
	for rows.Next() {
		var f domain.File
		if err := rows.Scan(&f.ID, &f.OwnerUserID, &f.Filename, &f.MimeType, &f.Kind, &f.SizeBytes, &f.StorageKey, &f.Status, &f.CreatedAt, &f.ReadyAt); err != nil {
			return nil, 0, err
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return files, total, nil
}

func (r *fileRepo) ListStalePending(ctx context.Context, olderThan time.Time) ([]domain.File, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, owner_user_id, filename, mime_type, kind, size_bytes, storage_key, status, created_at, ready_at
		FROM files
		WHERE status = 'pending' AND created_at < $1
	`, olderThan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []domain.File
	for rows.Next() {
		var f domain.File
		if err := rows.Scan(&f.ID, &f.OwnerUserID, &f.Filename, &f.MimeType, &f.Kind, &f.SizeBytes, &f.StorageKey, &f.Status, &f.CreatedAt, &f.ReadyAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return files, nil
}
