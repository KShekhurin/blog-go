-- +goose Up
CREATE TYPE outbox_message_type AS ENUM (
    'post.added',
    'post.deleted'
);

CREATE TABLE outbox (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    message_type outbox_message_type NOT NULL,
    payload      jsonb NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),

    processed_at timestamptz
);

CREATE INDEX idx_outbox_unprocessed
    ON outbox (created_at)
    WHERE processed_at IS NULL;

-- +goose Down
DROP INDEX idx_outbox_unprocessed;
DROP TABLE outbox;
DROP TYPE outbox_message_type;