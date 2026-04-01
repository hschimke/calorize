-- +goose Up
CREATE INDEX idx_users_name ON users(name);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_food_log_entries_food_id ON food_log_entries(food_id);
CREATE INDEX idx_food_log_entries_recent ON food_log_entries(user_id, food_id, logged_at);

-- +goose Down
DROP INDEX idx_food_log_entries_recent;
DROP INDEX idx_food_log_entries_food_id;
DROP INDEX idx_sessions_expires_at;
DROP INDEX idx_users_name;
