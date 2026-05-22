-- +goose Up
ALTER TABLE users ADD COLUMN weight_goal REAL;
ALTER TABLE users ADD COLUMN weight_unit TEXT NOT NULL DEFAULT 'kg';

CREATE TABLE weight_logs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    weight REAL NOT NULL,
    unit TEXT NOT NULL,
    logged_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_weight_logs_user_id_logged_at ON weight_logs(user_id, logged_at) WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX idx_weight_logs_user_id_logged_at;
DROP TABLE weight_logs;
-- SQLite does not support multiple drop columns in a single statement, so drop them separately
ALTER TABLE users DROP COLUMN weight_unit;
ALTER TABLE users DROP COLUMN weight_goal;
