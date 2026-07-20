-- name: FindPostById :one
SELECT * FROM posts
WHERE id = $1 LIMIT 1;

-- name: FindLinkedPostMedia :many
SELECT * FROM post_media
    WHERE post_id = $1;

-- name: AddPost :exec
INSERT INTO posts
    (id, author_id, reply_to, content, created_at, deleted_at)
VALUES
    ($1, $2, $3, $4, $5, $6);

-- name: AddPostMedia :exec
INSERT INTO post_media
    (id, post_id, type, mime_type, url, display_order)
VALUES
    ($1, $2, $3, $4, $5, $6);