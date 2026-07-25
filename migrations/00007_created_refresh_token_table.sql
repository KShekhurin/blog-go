-- +goose Up
CREATE TABLE allowed_refresh_tokens (
    jti uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    expires_at  timestamptz NOT NULL
);

CREATE INDEX idx_refresh_tokens_user_id ON allowed_refresh_tokens (user_id);

-- +goose Down
DROP TABLE IF EXISTS allowed_refresh_tokens;
DROP INDEX IF EXISTS idx_refresh_tokens_user_id;