-- +goose Up
CREATE TABLE new_food_log_entries (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    food_id TEXT REFERENCES foods(id) ON DELETE RESTRICT,
    calories REAL,
    amount REAL NOT NULL,
    meal_tag TEXT NOT NULL,
    logged_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP
);

INSERT INTO new_food_log_entries (id, user_id, food_id, amount, meal_tag, logged_at, created_at, deleted_at)
SELECT id, user_id, food_id, amount, meal_tag, logged_at, created_at, deleted_at FROM food_log_entries;

DROP TABLE food_log_entries;

ALTER TABLE new_food_log_entries RENAME TO food_log_entries;

CREATE INDEX idx_food_log_entries_user_id_logged_at ON food_log_entries(user_id, logged_at);

-- +goose Down
CREATE TABLE old_food_log_entries (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    food_id TEXT NOT NULL REFERENCES foods(id) ON DELETE RESTRICT,
    amount REAL NOT NULL,
    meal_tag TEXT NOT NULL,
    logged_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP
);

INSERT INTO old_food_log_entries (id, user_id, food_id, amount, meal_tag, logged_at, created_at, deleted_at)
SELECT id, user_id, food_id, amount, meal_tag, logged_at, created_at, deleted_at FROM food_log_entries WHERE food_id IS NOT NULL;

DROP TABLE food_log_entries;

ALTER TABLE old_food_log_entries RENAME TO food_log_entries;

CREATE INDEX idx_food_log_entries_user_id_logged_at ON food_log_entries(user_id, logged_at);
