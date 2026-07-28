-- name: FindPostById :one
SELECT * FROM posts
WHERE id = $1 LIMIT 1;

-- name: FindLinkedPostMedia :many
SELECT * FROM post_media
    WHERE post_id = $1;

-- name: FindUserPosts :many
SELECT * FROM posts
    WHERE author_id = $1
      AND deleted_at IS NULL
    ORDER BY created_at DESC, id DESC
    LIMIT $2;

-- name: FindUserPostsWithCursor :many
SELECT * FROM posts
    WHERE author_id = $1
      AND deleted_at IS NULL
      AND (created_at, id) < (sqlc.arg(last_created_at), sqlc.arg(last_id)::uuid)
    ORDER BY created_at DESC, id DESC
    LIMIT $2;

-- name: FindPostsMedias :many
SELECT * FROM post_media
    WHERE post_id = ANY($1::uuid[]);

-- name: FindPostsByIds :many
SELECT * FROM posts
    WHERE id = ANY($1::uuid[])
    AND deleted_at IS NULL;

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

-- name: DeletePost :execrows
UPDATE posts
SET deleted_at = $2
WHERE id = $1;