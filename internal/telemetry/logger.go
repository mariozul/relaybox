package telemetry

import (
	"log/slog"
	"os"
)

// NewLogger creates a new structured JSON logger writing to stdout.
func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}
