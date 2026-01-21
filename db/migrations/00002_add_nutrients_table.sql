CREATE TABLE IF NOT EXISTS food_nutrients (
    food_id TEXT NOT NULL,
    name TEXT NOT NULL,
    amount REAL NOT NULL,
    unit TEXT NOT NULL, -- e.g. 'g', 'mg', '%', 'mcg'
    PRIMARY KEY (food_id, name),
    FOREIGN KEY(food_id) REFERENCES foods(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_food_nutrients_food_id ON food_nutrients(food_id);
