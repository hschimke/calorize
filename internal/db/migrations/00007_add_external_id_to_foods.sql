-- +goose Up
-- +goose StatementBegin
ALTER TABLE foods ADD COLUMN external_id VARCHAR(255);
CREATE INDEX idx_foods_external_id ON foods(external_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX idx_foods_external_id;
ALTER TABLE foods DROP COLUMN external_id;
-- +goose StatementEnd
