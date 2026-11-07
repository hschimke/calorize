package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"azule.info/calorize/internal/auth"
	"azule.info/calorize/internal/db"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

type responseWriter struct {
	http.ResponseWriter
	status       int
	bytesWritten int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
}

// Logger middleware captures request details and logs them using structured slog.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{
			ResponseWriter: w,
			status:         http.StatusOK, // Default status
		}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)
		ctx := r.Context()
		reqID := GetRequestID(ctx)

		userIDStr := "anonymous"
		if v := ctx.Value(auth.UserIDContextKey); v != nil {
			switch id := v.(type) {
			case db.UserID:
				if id != db.UserID(uuid.Nil) {
					userIDStr = id.String()
				}
			case uuid.UUID:
				if id != uuid.Nil {
					userIDStr = id.String()
				}
			case string:
				if id != "" && id != uuid.Nil.String() {
					userIDStr = id
				}
			case fmt.Stringer:
				s := id.String()
				if s != "" && s != uuid.Nil.String() {
					userIDStr = s
				}
			default:
				s := fmt.Sprintf("%v", v)
				if s != "" && s != uuid.Nil.String() {
					userIDStr = s
				}
			}
		} else if cookie, err := r.Cookie(auth.AppSessionCookieName); err == nil && cookie.Value != "" {
			if session, sErr := db.GetSession(cookie.Value); sErr == nil && session != nil {
				userIDStr = session.UserID.String()
			}
		}

		traceIDStr := ""
		spanCtx := trace.SpanContextFromContext(ctx)
		if spanCtx.IsValid() {
			traceIDStr = spanCtx.TraceID().String()
		}

		slog.InfoContext(ctx, "http.request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", duration.Milliseconds(),
			"bytes_written", rw.bytesWritten,
			"request_id", reqID,
			"user_id", userIDStr,
			"trace_id", traceIDStr,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	})
}
