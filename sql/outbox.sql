-- name: AddToOutbox :exec
INSERT INTO outbox
    (message_type, payload)
VALUES
    ($1, $2);

-- name: GetOutboxEvents :many
SELECT * FROM outbox
WHERE processed_at IS NULL
ORDER BY created_at
LIMIT $1;

-- name: SetEventsAsProcessed :exec
UPDATE outbox
SET processed_at = now()
WHERE id = ANY($1::uuid[]);