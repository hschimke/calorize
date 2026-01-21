package api

import (
	"encoding/json"
	"net/http"
	"time"

	"azule.info/calorize/internal/database"
)

type StatsResponse struct {
	Period           string              `json:"period"`
	Start            time.Time           `json:"start"`
	End              time.Time           `json:"end"`
	TotalCalories    float64             `json:"total_calories"`
	TotalProtein     float64             `json:"total_protein"`
	TotalCarbs       float64             `json:"total_carbs"`
	TotalFat         float64             `json:"total_fat"`
	TotalNutrients   []database.Nutrient `json:"total_nutrients"`
	MacrosPercentage map[string]int      `json:"macros_percentage"`
}

func GetStats(w http.ResponseWriter, r *http.Request) {
	// Params
	period := r.URL.Query().Get("period") // day, week, month
	if period == "" {
		period = "day"
	}
	dateStr := r.URL.Query().Get("date") // YYYY-MM-DD

	// User ID
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = r.URL.Query().Get("user_id")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		return
	}

	// Calculate Range
	var start, end time.Time
	var err error

	now := time.Now()
	if dateStr != "" {
		start, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			http.Error(w, "invalid date format", http.StatusBadRequest)
			return
		}
	} else {
		// Default to today/now
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	}

	// Adjust for period
	switch period {
	case "day":
		// start is already 00:00 of the day
		end = start.Add(24 * time.Hour)
	case "week":
		// adjust start to Monday? Or just 7 days from date?
		// Standard: Week starting on that date? Or week containing that date?
		// Let's do: Start date IS the start of the week requested.
		end = start.Add(7 * 24 * time.Hour)
	case "month":
		// Start date -> +1 Month
		// Force start to 1st of month?
		// If user passes 2023-10-27 and says "month", do they mean "Oct 2023" or "30 days starting 27th"?
		// Standard convention: "Month of Oct 2023".
		// Reset start to first of month
		start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())
		end = start.AddDate(0, 1, 0)
	default:
		http.Error(w, "invalid period (day, week, month)", http.StatusBadRequest)
		return
	}

	stats, err := database.GetStats(r.Context(), userID, start, end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Calculate Percentages
	totalMacros := stats.TotalProtein + stats.TotalCarbs + stats.TotalFat
	pct := make(map[string]int)
	if totalMacros > 0 {
		pct["protein"] = int((stats.TotalProtein / totalMacros) * 100)
		pct["carbs"] = int((stats.TotalCarbs / totalMacros) * 100)
		pct["fat"] = int((stats.TotalFat / totalMacros) * 100)
	} else {
		pct["protein"] = 0
		pct["carbs"] = 0
		pct["fat"] = 0
	}

	resp := StatsResponse{
		Period:           period,
		Start:            start,
		End:              end,
		TotalCalories:    stats.TotalCalories,
		TotalProtein:     stats.TotalProtein,
		TotalCarbs:       stats.TotalCarbs,
		TotalFat:         stats.TotalFat,
		TotalNutrients:   stats.TotalNutrients,
		MacrosPercentage: pct,
	}

	json.NewEncoder(w).Encode(resp)
}
