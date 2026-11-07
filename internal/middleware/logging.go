package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"azule.info/calorize/internal/auth"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

type responseWriter struct {
	http.ResponseWriter
	status       int
	bytesWritten int64
	wroteHeader  bool
}

func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.wroteHeader = true
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
}

// Logger middleware captures request details and logs them using structured slog.
// The defer block guarantees that panicked or terminated requests are still logged.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{
			ResponseWriter: w,
			status:         http.StatusOK, // Default status
		}

		defer func() {
			duration := time.Since(start)
			ctx := r.Context()
			reqID := GetRequestID(ctx)

			userIDStr := "anonymous"
			if v := ctx.Value(auth.UserIDContextKey); v != nil {
				switch id := v.(type) {
				case uuid.UUID:
					if id != uuid.Nil {
						userIDStr = id.String()
					}
				case fmt.Stringer:
					s := id.String()
					if s != "" && s != uuid.Nil.String() {
						userIDStr = s
					}
				case string:
					if id != "" && id != uuid.Nil.String() {
						userIDStr = id
					}
				default:
					s := fmt.Sprintf("%v", v)
					if s != "" && s != uuid.Nil.String() {
						userIDStr = s
					}
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
		}()

		next.ServeHTTP(rw, r)
	})
}
