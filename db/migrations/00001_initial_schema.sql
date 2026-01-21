CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY, -- UUID
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    disabled_at DATETIME, -- If NULL, user is enabled
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_credentials (
    id TEXT PRIMARY KEY, -- Credential ID (WebAuthn credentialID)
    user_id TEXT NOT NULL,
    name TEXT, -- e.g. "My MacBook"
    public_key BLOB NOT NULL,
    attestation_type TEXT,
    aaguid TEXT,
    sign_count INTEGER NOT NULL DEFAULT 0,
    transports TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_used_at DATETIME,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS foods (
    id TEXT PRIMARY KEY, -- Version ID
    family_id TEXT NOT NULL, -- Shared across versions
    version INTEGER NOT NULL,
    is_current BOOLEAN NOT NULL DEFAULT 0,
    name TEXT NOT NULL,
    calories REAL NOT NULL,
    protein REAL NOT NULL,
    carbs REAL NOT NULL,
    fat REAL NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('food', 'recipe')),
    measurement_unit TEXT NOT NULL, -- e.g. 'g', 'ml', 'serving'
    measurement_amount REAL NOT NULL, -- e.g. 100
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE TABLE IF NOT EXISTS recipe_items (
    recipe_id TEXT NOT NULL, -- FK to foods.id (The Parent)
    ingredient_id TEXT NOT NULL, -- FK to foods.id (The Part)
    amount REAL NOT NULL,
    PRIMARY KEY(recipe_id, ingredient_id),
    FOREIGN KEY(recipe_id) REFERENCES foods(id) ON DELETE CASCADE,
    FOREIGN KEY(ingredient_id) REFERENCES foods(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS logs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    food_id TEXT NOT NULL, -- FK to Specific Version
    amount REAL NOT NULL,
    meal_tag TEXT NOT NULL, -- 'breakfast', 'lunch', etc.
    logged_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(food_id) REFERENCES foods(id) ON DELETE RESTRICT
);

-- Optimization Indices
CREATE INDEX IF NOT EXISTS idx_foods_family_id ON foods(family_id);
CREATE INDEX IF NOT EXISTS idx_foods_is_current ON foods(is_current);
CREATE INDEX IF NOT EXISTS idx_logs_user_id ON logs(user_id);
CREATE INDEX IF NOT EXISTS idx_logs_food_id ON logs(food_id);
CREATE INDEX IF NOT EXISTS idx_logs_logged_at ON logs(logged_at);

-- Views
CREATE VIEW IF NOT EXISTS logs_with_nutrients AS
SELECT 
    l.id AS log_id,
    l.user_id,
    l.food_id,
    f.name AS food_name,
    l.amount,
    f.measurement_unit,
    f.measurement_amount,
    l.meal_tag,
    l.logged_at,
    l.created_at,
    (l.amount / f.measurement_amount) * f.calories AS calories,
    (l.amount / f.measurement_amount) * f.protein AS protein,
    (l.amount / f.measurement_amount) * f.carbs AS carbs,
    (l.amount / f.measurement_amount) * f.fat AS fat
FROM logs l
JOIN foods f ON l.food_id = f.id
WHERE l.deleted_at IS NULL;

CREATE VIEW IF NOT EXISTS recipe_details AS
SELECT 
    ri.recipe_id,
    ri.ingredient_id,
    ri.amount AS ingredient_amount,
    f.name AS ingredient_name,
    f.calories AS ingredient_calories_unit, -- per f.measurement_amount
    f.measurement_unit AS ingredient_unit,
    f.measurement_amount AS ingredient_base_amount
FROM recipe_items ri
JOIN foods f ON ri.ingredient_id = f.id;
