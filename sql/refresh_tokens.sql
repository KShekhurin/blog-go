-- name: AddRefreshToken :exec
INSERT INTO allowed_refresh_tokens
    (jti, user_id, expires_at) VALUES ($1, $2, $3);

-- name: DeleteRefreshToken :execrows
DELETE FROM allowed_refresh_tokens
    WHERE jti = $1;

-- name: DeleteAllRefreshTokensByUserId :exec
DELETE FROM allowed_refresh_tokens
    WHERE user_id = $1;