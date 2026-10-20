// Package main starts the Relaybox HTTP service with liveness/readiness probes,
// structured JSON logging, and Prometheus metrics (ST-09: Observability &
// Health Endpoints).
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mariozul/relaybox/internal/version"
	"github.com/mariozul/relaybox/pkg/relaybox/handler"
	"github.com/mariozul/relaybox/pkg/relaybox/log"
	"github.com/mariozul/relaybox/pkg/relaybox/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// dbChecker implements handler.DBChecker to verify database reachability
// for the /readyz endpoint (FR-OBS-01, RULE-OBS-04).
type dbChecker struct{}

func (d *dbChecker) CheckDB(ctx context.Context) error {
	// TODO: ST-03/ST-04 - Implement actual database ping once the PostgreSQL
	// repository is wired in. For now, we assume the database is reachable.
	// This will be replaced with a proper sql.DB.PingContext() call.
	_ = ctx
	return nil
}

func main() {
	// RULE-OBS-01: Structured JSON logging via slog.
	logger := log.NewJSONLogger()

	// Register Prometheus metrics with the default registry (RULE-OBS-03).
	metrics.RegisterMetrics(prometheus.DefaultRegisterer)

	// Initialize the DBChecker for readiness probe.
	checker := &dbChecker{}

	// Initialize the HealthHandler.
	healthHandler := handler.NewHealthHandler(checker)

	mux := http.NewServeMux()

	// RULE-OBS-04: Liveness & readiness probes.
	mux.HandleFunc("/livez", healthHandler.HandleLivez)
	mux.HandleFunc("/readyz", healthHandler.HandleReadyz)

	// RULE-OBS-03: Expose Prometheus metrics endpoint.
	mux.Handle("/metrics", promhttp.Handler())

	// TODO: Wire ingestion and subscription handlers here (ST-06, ST-07).

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Propagate logger into the base context.
	ctx := log.WithContext(context.Background(), logger)

	go func() {
		logger.InfoContext(ctx, "relaybox starting",
			"version", version.Version,
			"addr", srv.Addr,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.ErrorContext(ctx, "server failed", "err", err)
			os.Exit(1)
		}
	}()

	// RULE-RES-02: Deterministic lifecycle via graceful shutdown.
	shutdownCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-shutdownCtx.Done()

	shutdownTimeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownTimeoutCtx); err != nil {
		logger.ErrorContext(ctx, "graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	logger.InfoContext(ctx, "relaybox stopped")
}
