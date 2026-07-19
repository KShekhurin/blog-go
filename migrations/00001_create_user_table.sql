-- +goose Up
CREATE TABLE users (
    id uuid primary key,
    login varchar(20) not null unique,
    email varchar(30) not null unique,
    password_hash varchar(100) not null
);

CREATE TABLE subscriber_author (
    sub_id uuid not null,
    auth_id uuid not null,

    primary key (sub_id, auth_id)
);

-- +goose Down
DROP TABLE users;
DROP TABLE subscriber_author;
