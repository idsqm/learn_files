-- +goose Up

CREATE TABLE IF NOT EXISTS files (
    id SERIAL PRIMARY KEY,
    owner_user_id UUID NOT NULL,
    filename VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    kind VARCHAR(20) NOT NULL,        -- video|document|image
    size_bytes BIGINT NOT NULL,
    storage_key VARCHAR(500) NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending|ready|failed
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ready_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_files_owner ON files(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_files_status_created ON files(status, created_at);

-- +goose Down
DROP TABLE IF EXISTS files CASCADE;
