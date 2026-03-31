-- +goose Up
CREATE TABLE user_disabled_sources (
    user_id TEXT NOT NULL REFERENCES users(id),
    source  TEXT NOT NULL,
    PRIMARY KEY (user_id, source)
);

-- +goose Down
DROP TABLE user_disabled_sources;
