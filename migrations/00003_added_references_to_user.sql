-- +goose Up
ALTER TABLE subscriber_author
    ADD CONSTRAINT fk_subscriber_user FOREIGN KEY (sub_id) REFERENCES users(id),
    ADD CONSTRAINT fk_author_user FOREIGN KEY (auth_id) REFERENCES users(id),
    ADD CONSTRAINT chk_sub_not_auth CHECK (sub_id <> auth_id);

-- +goose Down
ALTER TABLE subscriber_author
    DROP CONSTRAINT IF EXISTS fk_subscriber_user,
    DROP CONSTRAINT IF EXISTS fk_author_user,
    DROP CONSTRAINT IF EXISTS chk_sub_not_auth;
