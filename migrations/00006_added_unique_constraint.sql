-- +goose Up
ALTER TABLE post_media
    ADD CONSTRAINT unq_post_id_display_order UNIQUE (post_id, display_order) DEFERRABLE INITIALLY DEFERRED;

-- +goose Down
ALTER TABLE post_media
    DROP CONSTRAINT IF EXISTS unq_post_id_display_order;