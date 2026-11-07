package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"azule.info/calorize/internal/db"
	"azule.info/calorize/internal/middleware"
)

type ErrorResponse struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	RequestID string `json:"request_id,omitempty"`
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	reqID := middleware.GetRequestID(r.Context())
	status := http.StatusInternalServerError
	code := "INTERNAL_ERROR"
	msg := "An unexpected error occurred"

	switch {
	case errors.Is(err, db.ErrNotFound) || errors.Is(err, db.ErrCredentialNotFound):
		status = http.StatusNotFound
		code = "NOT_FOUND"
		msg = err.Error()
	case errors.Is(err, db.ErrConflict):
		status = http.StatusConflict
		code = "CONFLICT"
		msg = err.Error()
	case errors.Is(err, db.ErrInvalidInput):
		status = http.StatusBadRequest
		code = "BAD_REQUEST"
		msg = err.Error()
	default:
		slog.ErrorContext(r.Context(), "server error handling request",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
			"request_id", reqID,
		)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:     msg,
		Code:      code,
		RequestID: reqID,
	})
}
