-- +goose Up
ALTER TABLE foods ADD COLUMN brand_owner TEXT;
ALTER TABLE foods ADD COLUMN barcode TEXT;
ALTER TABLE foods ADD COLUMN ingredients_text TEXT;
ALTER TABLE foods ADD COLUMN category TEXT;

CREATE TABLE food_portions (
    food_id TEXT NOT NULL REFERENCES foods(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    amount REAL NOT NULL,
    unit TEXT,
    gram_weight REAL NOT NULL,
    PRIMARY KEY (food_id, name)
);

CREATE INDEX idx_foods_barcode ON foods(barcode);
CREATE INDEX idx_food_portions_food_id ON food_portions(food_id);

-- +goose Down
DROP INDEX idx_food_portions_food_id;
DROP INDEX idx_foods_barcode;
DROP TABLE food_portions;
ALTER TABLE foods DROP COLUMN category;
ALTER TABLE foods DROP COLUMN ingredients_text;
ALTER TABLE foods DROP COLUMN barcode;
ALTER TABLE foods DROP COLUMN brand_owner;
