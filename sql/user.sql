-- name: FindUserById :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: FindUserByLogin :one
SELECT * FROM users
WHERE login = $1 LIMIT 1;

-- name: FindUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: FindUserByLoginOrEmail :one
SELECT * FROM users
WHERE login = $1 OR email = $2 LIMIT 1;

-- name: Register :exec
INSERT INTO users (id, login, email, password_hash)
       VALUES ($1, $2, $3, $4);

-- name: SubscribeUserTo :exec
INSERT INTO subscriber_author (sub_id, auth_id)
       VALUES ($1, $2);

-- name: UnsubscribeUserFrom :exec
DELETE FROM subscriber_author
       WHERE sub_id = $1 AND auth_id = $2;