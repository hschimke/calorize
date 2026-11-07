package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"azule.info/calorize/internal/auth"
	"azule.info/calorize/internal/telemetry"
	"go.opentelemetry.io/otel/trace"
)

type routeKey struct{}

// routeHolder is shared by pointer through the request context so that
// RecordRoute calls nested under http.StripPrefix (which copies the request)
// can report the matched pattern back to Logger.
type routeHolder struct {
	pattern string
	set     bool
}

// RecordRoute looks up the pattern mux would match for r, records it (prefixed
// with the path prefix already stripped from r) for the enclosing Logger, then
// serves next. The innermost RecordRoute wins; a catch-all "/" match is
// recorded as unmatched. next is usually mux itself, or mux wrapped in other
// middleware such as RequireAuth.
func RecordRoute(prefix string, mux *http.ServeMux, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if holder, ok := r.Context().Value(routeKey{}).(*routeHolder); ok {
			_, pattern := mux.Handler(r)
			holder.pattern = joinPattern(prefix, pattern)
			holder.set = true
		}
		next.ServeHTTP(w, r)
	})
}

// joinPattern turns ("/api/v1", "GET /foods/{id}") into "GET /api/v1/foods/{id}".
// It returns "" for no match or a catch-all "/" match.
func joinPattern(prefix, pattern string) string {
	method, path, hasMethod := strings.Cut(pattern, " ")
	if !hasMethod {
		method, path = "", pattern
	}
	if path == "" || path == "/" {
		return ""
	}
	if method != "" {
		return method + " " + prefix + path
	}
	return prefix + path
}

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

		var panicked bool
		holder := &routeHolder{}
		r = r.WithContext(context.WithValue(r.Context(), routeKey{}, holder))

		defer func() {
			duration := time.Since(start)
			ctx := r.Context()

			// A panic unwinding through here will be turned into a 500 by an
			// outer Recoverer; record that rather than the default 200.
			status := rw.status
			if panicked && !rw.wroteHeader {
				status = http.StatusInternalServerError
			}
			reqID := GetRequestID(ctx)

			userIDStr := "anonymous"
			if uid, ok := auth.UserIDFromContext(ctx); ok {
				userIDStr = uid.String()
			}

			traceIDStr := ""
			spanCtx := trace.SpanContextFromContext(ctx)
			if spanCtx.IsValid() {
				traceIDStr = spanCtx.TraceID().String()
			}

			slog.InfoContext(ctx, "http.request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"duration_ms", duration.Milliseconds(),
				"bytes_written", rw.bytesWritten,
				"request_id", reqID,
				"user_id", userIDStr,
				"trace_id", traceIDStr,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)

			route := r.Pattern
			if holder.set {
				route = holder.pattern
			}
			if route == "" {
				route = "unmatched"
			}
			telemetry.RecordHTTPRequest(r.Method, route, status, duration.Seconds(), rw.bytesWritten)
		}()

		panicked = true
		next.ServeHTTP(rw, r)
		panicked = false
	})
}
