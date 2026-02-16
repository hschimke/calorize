-- +goose Up
ALTER TABLE foods ADD COLUMN servings REAL NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE foods DROP COLUMN servings;
