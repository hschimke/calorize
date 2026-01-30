package api

import "net/http"

// ### Foods
// - GET /foods
//     - Returns list of current versions
// - POST /foods
//     - Create new food/recipe
//     - Payload: { name, calories, protein, carbs, fat, type, measurement_unit, measurement_amount, nutrients: [], ingredients: {} }
// - GET /foods/{id}
//     - Returns details including sub-ingredients if recipe
// - PUT /foods/{id}
//     - Updates a food by creating a NEW Version
//     - Payload: Same as POST
// - DELETE /foods/{id}
//     - Soft delete

// ### Logs
// - GET /logs
//     - Query Params: ?date=YYYY-MM-DD (Defaults to today)
//     - Returns logs for the day
// - POST /logs
//     - Create log entry
//     - Payload: { food_id, amount, meal_tag, logged_at (optional) }
// - DELETE /logs/{id}

// ### Stats
// - GET /stats
//     - Query Params: ?period={day,week,month}&date=YYYY-MM-DD
//     - Returns aggregated macros and total calories

func RegisterApiPaths(mux *http.ServeMux) {}
