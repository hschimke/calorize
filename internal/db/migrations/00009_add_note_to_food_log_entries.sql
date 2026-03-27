-- +goose Up
ALTER TABLE food_log_entries ADD COLUMN note TEXT;

-- +goose Down
ALTER TABLE food_log_entries DROP COLUMN note;
