package db

import "errors"

var (
	ErrNotFound     = errors.New("record not found")
	ErrConflict     = errors.New("resource conflict")
	ErrInvalidInput = errors.New("invalid input")
	ErrDatabase     = errors.New("database operation failed")
)
