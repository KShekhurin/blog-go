-- +goose Up
CREATE INDEX created_at_id_index ON posts (created_at, id);

-- +goose Down
DROP INDEX created_at_id_index;
