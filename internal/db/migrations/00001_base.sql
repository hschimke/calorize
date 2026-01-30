-- +goose Up
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    disabled_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE user_credentials (
    id BLOB PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    public_key BLOB NOT NULL,
    attestation_type TEXT NOT NULL,
    aaguid TEXT NOT NULL,
    sign_count INTEGER NOT NULL,
    transports TEXT,
    backup_eligible BOOLEAN NOT NULL,
    backup_state BOOLEAN NOT NULL,
    created_at TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP NOT NULL
);

CREATE TABLE foods (
    id TEXT PRIMARY KEY,
    creator_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
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
    public BOOLEAN NOT NULL,
    created_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP
);

CREATE TABLE food_nutrients (
    food_id TEXT NOT NULL REFERENCES foods(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    amount REAL NOT NULL,
    unit TEXT NOT NULL,
    PRIMARY KEY (food_id, name)
);

CREATE TABLE recipe_items (
    recipe_id TEXT NOT NULL REFERENCES foods(id) ON DELETE CASCADE,
    ingredient_id TEXT NOT NULL REFERENCES foods(id) ON DELETE RESTRICT,
    amount REAL NOT NULL,
    PRIMARY KEY (recipe_id, ingredient_id)
);

CREATE TABLE food_log_entries (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    food_id TEXT NOT NULL REFERENCES foods(id) ON DELETE RESTRICT,
    amount REAL NOT NULL,
    meal_tag TEXT NOT NULL,
    logged_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_user_credentials_user_id ON user_credentials(user_id);
CREATE INDEX idx_foods_family_id ON foods(family_id);
CREATE INDEX idx_foods_name ON foods(name);
CREATE INDEX idx_foods_creator_id ON foods(creator_id);
CREATE INDEX idx_recipe_items_ingredient_id ON recipe_items(ingredient_id);
CREATE INDEX idx_food_log_entries_user_id_logged_at ON food_log_entries(user_id, logged_at);

-- +goose Down
DROP INDEX idx_food_log_entries_user_id_logged_at;
DROP INDEX idx_recipe_items_ingredient_id;
DROP INDEX idx_foods_name;
DROP INDEX idx_foods_family_id;
DROP INDEX idx_foods_creator_id;
DROP INDEX idx_user_credentials_user_id;

DROP TABLE food_log_entries;
DROP TABLE recipe_items;
DROP TABLE food_nutrients;
DROP TABLE foods;
DROP TABLE user_credentials;
DROP TABLE users;