-- +goose Up
SET TIMEZONE='UTC';

CREATE TYPE media_type AS ENUM (
    'image',
    'video',
    'audio',
    'document'
);

CREATE TABLE posts (
    id uuid primary key,
    author_id uuid not null,
    reply_to uuid references posts(id),

    content text not null,
    created_at timestamptz not null,

    is_deleted bool not null default false,
    deleted_at timestamptz default null
);

CREATE TABLE post_media (
    id uuid primary key,
    post_id uuid not null references posts(id),

    type media_type not null,
    mime_type varchar(30) not null,

    url text not null,
    display_order int not null
);

-- +goose Down
DROP TABLE post_media;
DROP TABLE posts;
DROP TYPE media_type;