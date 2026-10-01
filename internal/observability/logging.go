package observability

import (
	"log/slog"
	"os"
)

// NewLogger creates a structured JSON logger writing to stdout (RULE-OBS-01).
func NewLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}
