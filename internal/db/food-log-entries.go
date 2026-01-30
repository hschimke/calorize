package db

import "time"

func GetFoodLogEntries(userID UserID, date time.Time) ([]FoodLogEntry, error) {
	panic("not implemented")
}

func CreateFoodLogEntry(entry FoodLogEntry) (*FoodLogEntry, error) {
	panic("not implemented")
}

func UpdateFoodLogEntry(entry FoodLogEntry) (*FoodLogEntry, error) {
	panic("not implemented")
}

func DeleteFoodLogEntry(entry FoodLogEntry) error {
	panic("not implemented")
}
