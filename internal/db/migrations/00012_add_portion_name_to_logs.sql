-- +goose Up
ALTER TABLE food_log_entries ADD COLUMN portion_name TEXT;

-- +goose Down
ALTER TABLE food_log_entries DROP COLUMN portion_name;
