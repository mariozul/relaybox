package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/mariozul/relaybox/internal/dispatcher"
	"github.com/mariozul/relaybox/internal/httpapi"
	"github.com/mariozul/relaybox/internal/observability"
	"github.com/mariozul/relaybox/internal/postgres"
	"github.com/mariozul/relaybox/internal/version"
	"github.com/mariozul/relaybox/internal/webhook"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	store, err := postgres.Open(ctx, required("DATABASE_URL"))
	if err != nil {
		logger.ErrorContext(ctx, "database startup failed", "error", err)
		return
	}
	defer store.Close()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	registry := prometheus.NewRegistry()
	metrics := observability.New(registry)
	api := httpapi.New(store, required("RELAYBOX_AUTH_TOKEN"))
	mux := http.NewServeMux()
	mux.Handle("/v1/", metrics.Handler("/v1", api))
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		checkCtx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if store.Ping(checkCtx) != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	delivery := dispatcher.New(store, webhook.NewSender(&http.Client{Timeout: 10 * time.Second}), dispatcher.Config{Workers: envInt("WORKERS", 4), MaxAttempts: envInt("MAX_ATTEMPTS", 5)})
	go func() {
		if err := delivery.Run(ctx); err != nil && ctx.Err() == nil {
			logger.ErrorContext(ctx, "dispatcher failed", "error", err)
			stop()
		}
	}()
	srv := &http.Server{Addr: env("ADDR", ":8080"), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		logger.InfoContext(ctx, "relaybox starting", "version", version.Version, "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.ErrorContext(ctx, "server failed", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.ErrorContext(ctx, "shutdown failed", "error", err)
	}
}

func required(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(key + " is required")
	}
	return value
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}
