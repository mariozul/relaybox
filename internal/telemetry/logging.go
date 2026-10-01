package telemetry

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

// RULE-OBS-01: structured JSON logging bound to ctx.
func NewLogger(level slog.Level) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return a
			}
			return a
		},
	})
	return slog.New(handler)
}

// WithContext enriches a logger with trace/span IDs from OpenTelemetry ctx.
func LoggerWithTrace(ctx context.Context, logger *slog.Logger) *slog.Logger {
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		return logger
	}
	return logger.With(
		"trace_id", spanCtx.TraceID().String(),
		"span_id", spanCtx.SpanID().String(),
	)
}
