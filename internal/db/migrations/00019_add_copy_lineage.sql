-- +goose Up
ALTER TABLE foods ADD COLUMN copied_from_id TEXT REFERENCES foods(id);
ALTER TABLE food_log_entries ADD COLUMN copied_from_id TEXT REFERENCES food_log_entries(id);
CREATE INDEX idx_foods_copied_from_id ON foods(copied_from_id) WHERE copied_from_id IS NOT NULL;

-- +goose Down
DROP INDEX idx_foods_copied_from_id;
ALTER TABLE foods DROP COLUMN copied_from_id;
ALTER TABLE food_log_entries DROP COLUMN copied_from_id;
