package observability

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Logger wraps *slog.Logger for structured JSON logging with context
// propagation (RULE-OBS-01/02).
type Logger struct{ *slog.Logger }

// LoggerOption configures a Logger.
type LoggerOption func(*slog.HandlerOptions)

// WithLevel sets the minimum log level.
func WithLevel(level slog.Level) LoggerOption {
	return func(opts *slog.HandlerOptions) { opts.Level = level }
}

// NewLogger creates a structured JSON logger writing to w (default os.Stdout, Info level).
func NewLogger(w io.Writer, opts ...LoggerOption) *Logger {
	if w == nil {
		w = os.Stdout
	}
	ho := &slog.HandlerOptions{Level: slog.LevelInfo}
	for _, o := range opts {
		o(ho)
	}
	return &Logger{Logger: slog.New(slog.NewJSONHandler(w, ho))}
}

func (l *Logger) withTrace(ctx context.Context, msg string, args []any) []any {
	tid, sid := ExtractTraceContext(ctx)
	return append([]any{"trace_id", tid, "span_id", sid}, args...)
}

// InfoContext logs at Info with trace context (RULE-OBS-01).
func (l *Logger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.Logger.InfoContext(ctx, msg, l.withTrace(ctx, msg, args)...)
}

// ErrorContext logs at Error with trace context (RULE-OBS-01).
func (l *Logger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.Logger.ErrorContext(ctx, msg, l.withTrace(ctx, msg, args)...)
}

// WarnContext logs at Warn with trace context.
func (l *Logger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.Logger.WarnContext(ctx, msg, l.withTrace(ctx, msg, args)...)
}

// DebugContext logs at Debug with trace context.
func (l *Logger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.Logger.DebugContext(ctx, msg, l.withTrace(ctx, msg, args)...)
}

// ExtractTraceContext extracts OTel trace/span IDs from ctx. Returns empty
// strings if no valid span is active.
func ExtractTraceContext(ctx context.Context) (traceID, spanID string) {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if !sc.IsValid() {
		return "", ""
	}
	return sc.TraceID().String(), sc.SpanID().String()
}

// InjectTraceContext injects OTel trace context into HTTP headers for
// outbound calls (RULE-OBS-02).
func InjectTraceContext(ctx context.Context, headers http.Header) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(headers))
}

// HealthChecker checks a dependency's health. Returns nil if healthy;
// must respect ctx cancellation.
type HealthChecker func(ctx context.Context) error

// LivezHandler returns an http.Handler for the liveness probe (RULE-OBS-04).
func LivezHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
}

// ReadyzHandler returns an http.Handler for the readiness probe (RULE-OBS-04).
// Returns 200 if all checkers pass (or none registered), 503 if any fail.
func ReadyzHandler(checkers map[string]HealthChecker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var unhealthy []string
		for name, check := range checkers {
			if check != nil && check(ctx) != nil {
				unhealthy = append(unhealthy, name)
			}
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if len(unhealthy) > 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready\n"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready\n"))
	})
}
