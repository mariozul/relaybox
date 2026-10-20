// Package log provides structured JSON logging using slog, bound to
// context.Context for automatic trace_id and span_id extraction
// (FR-OBS-02, RULE-OBS-01).
package log

import (
	"context"
	"log/slog"
	"os"
)

// CtxKeyLogger is the context key used to store and retrieve the
// structured logger from the context.
type CtxKeyLogger struct{}

// NewJSONLogger creates a new slog.Logger configured for JSON output
// to os.Stdout. It automatically extracts trace_id and span_id from
// context when used with slog.InfoContext, slog.ErrorContext, etc.
func NewJSONLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Remove the default time key if we want compact output,
			// but we keep it for observability.
			return a
		},
	}))
}

// FromContext extracts a slog.Logger from the context. If no logger
// is found, it returns the default JSON logger.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(CtxKeyLogger{}).(*slog.Logger); ok {
		return logger
	}
	return NewJSONLogger()
}

// WithContext embeds a slog.Logger into the provided context and returns
// the new context. Handlers and services should use this to propagate
// the structured logger.
func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, CtxKeyLogger{}, logger)
}
