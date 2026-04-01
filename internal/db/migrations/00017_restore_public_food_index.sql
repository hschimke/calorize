-- +goose Up
CREATE INDEX idx_foods_public_is_current ON foods(public, is_current);

-- +goose Down
DROP INDEX idx_foods_public_is_current;
