-- +goose NO TRANSACTION
-- +goose Up
-- +goose StatementBegin
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE foods_new (
    id TEXT PRIMARY KEY,
    creator_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    family_id TEXT NOT NULL,
    version INTEGER NOT NULL,
    is_current BOOLEAN NOT NULL,
    name TEXT NOT NULL,
    calories REAL NOT NULL,
    protein REAL NOT NULL,
    carbs REAL NOT NULL,
    fat REAL NOT NULL,
    type TEXT NOT NULL,
    measurement_unit TEXT NOT NULL,
    measurement_amount REAL NOT NULL,
    servings REAL NOT NULL DEFAULT 1,
    public BOOLEAN NOT NULL,
    external_id VARCHAR(255),
    created_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP
);

INSERT INTO foods_new (id, creator_id, family_id, version, is_current, name, calories, protein, carbs, fat, type, measurement_unit, measurement_amount, servings, public, external_id, created_at, deleted_at)
SELECT id, CASE WHEN creator_id = '00000000-0000-0000-0000-000000000000' THEN NULL ELSE creator_id END, family_id, version, is_current, name, calories, protein, carbs, fat, type, measurement_unit, measurement_amount, servings, public, external_id, created_at, deleted_at 
FROM foods;

DROP TABLE foods;
ALTER TABLE foods_new RENAME TO foods;

CREATE INDEX idx_foods_family_id ON foods(family_id);
CREATE INDEX idx_foods_name ON foods(name);
CREATE INDEX idx_foods_creator_id ON foods(creator_id);
CREATE INDEX idx_foods_external_id ON foods(external_id);

COMMIT;
PRAGMA foreign_keys=ON;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Reverting requires ensuring no NULLs exist in creator_id, skipped for simplicity as system foods will break it
-- +goose StatementEnd
