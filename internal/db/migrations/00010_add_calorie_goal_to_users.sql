-- +goose Up
ALTER TABLE users ADD COLUMN calorie_goal INTEGER;

-- +goose Down
ALTER TABLE users DROP COLUMN calorie_goal;
