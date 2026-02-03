-- +goose Up
ALTER TABLE food_log_entries ADD COLUMN protein REAL;
ALTER TABLE food_log_entries ADD COLUMN carbs REAL;
ALTER TABLE food_log_entries ADD COLUMN fat REAL;

UPDATE food_log_entries
SET
    protein = (SELECT (food_log_entries.amount / CASE WHEN f.measurement_amount = 0 THEN 1 ELSE f.measurement_amount END) * f.protein FROM foods f WHERE f.id = food_log_entries.food_id),
    carbs = (SELECT (food_log_entries.amount / CASE WHEN f.measurement_amount = 0 THEN 1 ELSE f.measurement_amount END) * f.carbs FROM foods f WHERE f.id = food_log_entries.food_id),
    fat = (SELECT (food_log_entries.amount / CASE WHEN f.measurement_amount = 0 THEN 1 ELSE f.measurement_amount END) * f.fat FROM foods f WHERE f.id = food_log_entries.food_id)
WHERE food_id IS NOT NULL;

CREATE INDEX idx_foods_public_is_current ON foods(public, is_current);

-- +goose Down
DROP INDEX idx_foods_public_is_current;
ALTER TABLE food_log_entries DROP COLUMN protein;
ALTER TABLE food_log_entries DROP COLUMN carbs;
ALTER TABLE food_log_entries DROP COLUMN fat;
