-- +goose Up
-- Create a case-insensitive index for the SearchFoods LIKE prefix query.
-- Keeping the original idx_foods_name intact (which is case sensitive).
CREATE INDEX idx_foods_name_nocase ON foods(name COLLATE NOCASE);

-- +goose Down
DROP INDEX idx_foods_name_nocase;
