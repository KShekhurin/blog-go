-- +goose Up
ALTER TABLE posts
DROP COLUMN IF EXISTS is_deleted;

-- +goose Down
ALTER TABLE posts
ADD COLUMN IF NOT EXISTS is_deleted bool not null default false
