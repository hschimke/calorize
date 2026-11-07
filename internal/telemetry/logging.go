package telemetry

import (
	"io"
	"log/slog"
)

// NewLogHandler returns the JSON slog handler the API server logs with.
//
// The message is written under "message" (not slog's default "msg"), so no
// log attribute may itself be named "message": it would produce a JSON object
// with a duplicate key, and log pipelines (Alloy, Loki's json parser) keep
// only one of the two values.
func NewLogHandler(w io.Writer) slog.Handler {
	return slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		// Adds "source":{"function":"...","file":"...","line":...} to trace log origin
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.MessageKey {
				a.Key = "message"
			}
			return a
		},
	})
}
