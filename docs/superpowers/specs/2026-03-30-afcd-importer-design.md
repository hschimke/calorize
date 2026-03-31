# AFCD Importer Design

**Date:** 2026-03-30
**Status:** Approved

## Overview

Import the Australian Food Composition Database (AFCD Release 3) into Calorize as a set of public, system-owned food records. The dataset contains 1,589 generic Australian foods with rich nutrient data (up to 272 nutrients per food, all per 100g).

This is a companion to the existing FDC importer. The two datasets coexist in the database — there is no programmatic merging possible, as AFCD uses no barcodes, brand names, or shared identifiers with FDC.

## Scope

- Import base foods only — **skip recipe reconstruction** (the Recipes xlsx file is methodology documentation; all composite foods already have pre-calculated nutrient values in the Nutrient Profiles sheet)
- Import all available micronutrients, **skipping null/zero values** to keep `food_nutrients` lean
- Idempotent: safe to re-run; creates a new food version only if data has changed

## Architecture

### New files

- `cmd/afcd-importer/main.go` — entry point; reads env vars, initialises DB, calls importer
- `internal/importer/afcd.go` — core import logic

### Modified files

- `docker/Dockerfile.api` — build and copy the `afcd-importer` binary
- `go.mod` / `go.sum` — add `github.com/xuri/excelize/v2` for xlsx parsing

### No changes needed

- `docker/docker-compose.yml` — uses existing `calorize-api` service with `docker-compose run`
- Database schema — all required fields already exist

## Running the Import

1. Copy the six AFCD xlsx files into `$MAPDIR/afcd/` on the host (appears as `/data/afcd/` in container)
2. Run:
   ```
   docker-compose run --rm -e AFCD_DIR=/data/afcd calorize-api /app/afcd-importer
   ```

### Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `DB_PATH` | `./test.db` | SQLite database path (same as api-server) |
| `AFCD_DIR` | `./aus` | Directory containing the xlsx files (default suits local dev) |

Note: in Docker, `DB_PATH` is already set to `/data/calorize.db` by `docker-compose.yml`. `AFCD_DIR` must be passed explicitly as `/data/afcd/` since the container working directory is `/app`, not `/data`.

## Data Mapping

All nutrient data in AFCD is expressed per 100g of food.

| AFCD field | DB field | Notes |
|---|---|---|
| Public Food Key | `external_id` | Format: `"afcd_" + PublicFoodKey` (e.g. `afcd_F002258`) |
| Food Name | `name` | |
| Food Description | `ingredients_text` | Best available fit for this field |
| Classification → Food Group name | `category` | Resolved via Food Group Info sheet |
| Energy with dietary fibre, equated (kJ) ÷ 4.184 | `calories` | kJ → kcal conversion; zero-calorie foods are inserted with a warning log |
| Protein (g) | `protein` | |
| Fat, total (g) | `fat` | |
| Available carbohydrates without sugar alcohols (g) | `carbs` | Standard labelling value |
| — | `measurement_unit` | Always `"g"` |
| — | `measurement_amount` | Always `100` |
| — | `servings` | Always `1` |
| — | `public` | Always `true` |
| — | `creator_id` | Always `NULL` (system food) |
| — | `type` | Always `"food"` |
| All other non-null/non-zero columns | `food_nutrients` | Full import (~270 nutrients) |

### food_nutrients rows

Each non-null, non-zero nutrient column from the Nutrient Profiles sheet produces one row:
- `food_id` — the new food version's UUID
- `name` — AFCD column header (e.g., `"Calcium (mg)"`)
- `amount` — numeric value
- `unit` — parenthetical suffix parsed from column header (e.g., `"mg"` from `"Calcium (mg)"`); use `""` (empty string) when no unit is present in the header (e.g. ratio or percentage columns)

**Important:** nutrient rows are always inserted fresh under the new food version's UUID. They are never upserted against existing rows. Because `food_nutrients` has `ON DELETE CASCADE` on `food_id`, old nutrient rows are automatically removed when an old food version is superseded by `UpdateFood()`.

## Import Logic

```
1. Parse Food Group Info sheet → map[classificationCode]categoryName
2. Parse Nutrient Profiles sheet → map[PublicFoodKey]NutrientRow
3. For each row in Food Details sheet:
   a. Look up nutrients from map (skip food if not found in nutrient profiles)
   b. Convert energy kJ → kcal; log warning if result is zero
   c. Resolve category from classification code (leave empty string if not found)
   d. Upsert the food (same pattern as fdc.go):
      - GetFoodByExternalID("afcd_" + PublicFoodKey)
      - If not found: CreateFood() — inserts food row + nutrient rows
      - If found: compare name, calories, protein, fat, carbs, category,
        ingredients_text; if any changed, UpdateFood() — old nutrient rows
        are cascade-deleted, new rows inserted under the new food UUID
      - If unchanged: skip
   e. (No separate nutrient upsert step — nutrient rows are created inside
      CreateFood()/UpdateFood() as part of the same transaction)
4. Log progress every 100 records; report inserted/updated/skipped/error counts
```

## Dockerfile Changes

In the builder stage of `docker/Dockerfile.api`, add after the existing `go build`:
```dockerfile
RUN CGO_ENABLED=0 GOOS=linux go build -o afcd-importer ./cmd/afcd-importer/
```

In the final stage, copy the binary:
```dockerfile
COPY --from=builder /app/afcd-importer .
```

## Verification

1. Copy xlsx files to `$MAPDIR/afcd/` (or `./aus/` for local dev)
2. Run the importer (Docker or local)
3. Confirm output: ~1,589 inserted, 0 errors
4. Query the DB: `SELECT count(*) FROM foods WHERE external_id LIKE 'afcd_%'` → should return ~1,589
5. Spot-check a food: `SELECT * FROM foods WHERE external_id = 'afcd_F002258'`
6. Check micronutrients: `SELECT count(*) FROM food_nutrients fn JOIN foods f ON f.id = fn.food_id WHERE f.external_id LIKE 'afcd_%'`
7. Re-run the importer → confirm all records show as skipped (no changes detected)
8. DB file is accessible on host at `$MAPDIR/calorize.db` without issue
