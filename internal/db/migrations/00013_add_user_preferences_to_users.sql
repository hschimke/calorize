-- +goose Up
ALTER TABLE users ADD COLUMN clown_mode BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN hide_public_user_foods BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE users DROP COLUMN hide_public_user_foods;
ALTER TABLE users DROP COLUMN clown_mode;
