# AFCD Importer Documentation

## Overview

The **Australian Food Composition Database (AFCD) Release 3** is a comprehensive food composition database maintained by Food Standards Australia New Zealand (FSANZ). The AFCD importer tool reads the three AFCD xlsx files and imports their food composition data into the Calorize database, creating public food records with complete nutrient information.

## Dataset

The AFCD Release 3 dataset contains **1,589 generic Australian foods** with **272 micronutrients** measured per food, normalized to **per 100g basis**. The database includes no barcodes or brand-specific data—it is a reference dataset of unbranded, generic food items suitable for public use.

### Required Files

The importer expects these three xlsx files in the import directory:

1. **AFCD Release 3 - Food group information.xlsx** — Maps classification codes to food group category names
2. **AFCD Release 3 - Nutrient profiles.xlsx** — Energy, protein, fat, carbohydrates, and all 268 micronutrient values per 100g
3. **AFCD Release 3 - Food Details.xlsx** — Food names, descriptions, and classification codes

## Data Mapping

The importer maps AFCD columns to Calorize database fields as follows:

| AFCD Field | Database Field | Notes |
|---|---|---|
| Public Food Key | external_id | Prefixed with `afcd_` (e.g., `afcd_F002258`) |
| Food Name | name | Exact name from AFCD |
| Food Description | ingredients_text | Stored as-is |
| Classification | category | Looked up from food group info file |
| Energy with dietary fibre, equated (kJ) | calories | Converted to kcal: `round((kJ / 4.184) × 100) / 100`, rounded to 2 decimal places |
| Protein (g) | protein | Per 100g |
| Fat, total (g) | fat | Per 100g |
| Available carbohydrates without sugar alcohols (g) | carbs | Per 100g |
| All other nutrient columns | food_nutrients table | Stored as micronutrients with name, amount, and extracted unit from header (e.g., "Calcium (mg)" → unit "mg") |
| — | measurement_unit | Hardcoded as `g` |
| — | measurement_amount | Hardcoded as `100` (per 100g) |
| — | type | Hardcoded as `food` |
| — | public | Hardcoded as `true` |

## Running Locally

To import AFCD data into a local SQLite database:

```bash
AFCD_DIR=./aus DB_PATH=./test.db go run ./cmd/afcd-importer/
```

Environment variables:
- `AFCD_DIR` — Path to directory containing the three xlsx files (defaults to `./aus`)
- `DB_PATH` — Path to SQLite database (defaults to `./test.db`)

The importer will log progress every 100 foods and display final counts of inserted, updated, skipped, and error records.

## Running in Docker

When using docker-compose, AFCD files are mounted from the host and placed in a specific container path:

1. Place the three AFCD xlsx files in `$MAPDIR/afcd/` on the host (e.g., `~/docker-dirs/calorize.azule.info/afcd/`)
2. Run the importer in a one-off container:

```bash
docker-compose run --rm -e AFCD_DIR=/data/afcd calorize-api /app/afcd-importer
```

The `$MAPDIR` environment variable (default: `~/docker-dirs/calorize.azule.info/`) mounts as `/data` inside the container, so xlsx files in `$MAPDIR/afcd/` appear as `/data/afcd/` to the importer.

## Idempotency

The importer is **safe to re-run** without duplication or data loss:

- Foods are identified by their `external_id` (the AFCD Public Food Key prefixed with `afcd_`)
- On each run, the importer checks if a food with that `external_id` already exists
- If the food exists but its data has changed (name, calories, protein, fat, carbs, category, description, or nutrient count), a new versioned copy is created via `UpdateFood`
- If nothing has changed, the food is skipped
- Calorize's food versioning system ensures only the latest version has `is_current=true`; previous versions are retained for historical log entries

## Verifying the Import

After importing, verify the data with these SQLite queries:

**Count total AFCD foods:**
```sql
SELECT COUNT(*) as afcd_foods FROM foods WHERE external_id LIKE 'afcd_%' AND is_current = true;
```

**Spot-check a single food (e.g., a vegetable):**
```sql
SELECT
  id, name, calories, protein, fat, carbs, category, measurement_unit, measurement_amount
FROM foods
WHERE external_id LIKE 'afcd_%' AND is_current = true
LIMIT 1;
```

**Count micronutrients imported:**
```sql
SELECT COUNT(*) as micronutrient_rows FROM food_nutrients
WHERE food_id IN (SELECT id FROM foods WHERE external_id LIKE 'afcd_%');
```

**Find foods with zero calories (data quality check):**
```sql
SELECT id, name, external_id FROM foods
WHERE external_id LIKE 'afcd_%' AND is_current = true AND calories = 0;
```

The importer logs all zero-calorie foods as warnings during the run; these are likely data issues in the AFCD source.
