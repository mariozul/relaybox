// Package main starts the Relaybox HTTP service with liveness/readiness probes,
// structured JSON logging, and Prometheus metrics (ST-09: Observability &
// Health Endpoints).
package main

import (
	"context"
	"database/sql"
	"log/slog"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mariozul/relaybox/internal/version"
	"github.com/mariozul/relaybox/pkg/relaybox/adapter/postgresql"
	"github.com/mariozul/relaybox/pkg/relaybox/handler"
	"github.com/mariozul/relaybox/pkg/relaybox/log"
	"github.com/mariozul/relaybox/pkg/relaybox/metrics"
	"github.com/mariozul/relaybox/pkg/relaybox/repository"
	"github.com/mariozul/relaybox/pkg/relaybox/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// config holds the application configuration loaded from command-line flags
// or environment variables.
type config struct {
	addr        string
	dbDSN       string
	readTimeout time.Duration
}

// parseConfig parses configuration from flags and environment variables.
func parseConfig() config {
	cfg := config{
		addr:        ":8080",
		dbDSN:       "",
		readTimeout: 5 * time.Second,
	}

	flag.StringVar(&cfg.addr, "addr", cfg.addr, "HTTP server address")
	flag.StringVar(&cfg.dbDSN, "db-dsn", cfg.dbDSN, "PostgreSQL connection DSN")
	flag.DurationVar(&cfg.readTimeout, "read-timeout", cfg.readTimeout, "HTTP read header timeout")
	flag.Parse()

	// Allow environment variable overrides.
	if v := os.Getenv("RELAYBOX_ADDR"); v != "" {
		cfg.addr = v
	}
	if v := os.Getenv("RELAYBOX_DB_DSN"); v != "" {
		cfg.dbDSN = v
	}

	return cfg
}

// openDB opens a PostgreSQL connection pool and verifies connectivity. Returns
// nil if dbDSN is empty (development mode without database).
func openDB(ctx context.Context, dbDSN string) (*sql.DB, error) {
	if dbDSN == "" {
		return nil, nil
	}

	db, err := sql.Open("postgres", dbDSN)
	if err != nil {
		return nil, err
	}

	// Verify database connectivity with a short timeout.
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// newDBHealthChecker creates a handler.DBChecker from a database connection.
// If db is nil, it returns nil, causing /readyz to return 503 until the
// database is configured.
func newDBHealthChecker(db *sql.DB) handler.DBChecker {
	if db == nil {
		return nil
	}
	return postgresql.NewDBChecker(db)
}

// run is the main application entry point. It performs initialization and
// runs the HTTP server, returning an error if startup or shutdown fails.
func run(ctx context.Context, logger *slog.Logger) error {
	cfg := parseConfig()

	// Open database connection (optional in development mode).
	db, err := openDB(ctx, cfg.dbDSN)
	if err != nil {
		logger.ErrorContext(ctx, "failed to connect to database",
			"error", err,
		)
		return err
	}
	defer func() {
		if db != nil {
			db.Close()
		}
	}()

	// RULE-OBS-01: Structured JSON logging via slog.
	logger.InfoContext(ctx, "relaybox starting",
		"version", version.Version,
		"addr", cfg.addr,
		"db_configured", db != nil,
	)

	// Register Prometheus metrics with the default registry (RULE-OBS-03).
	metrics.RegisterMetrics(prometheus.DefaultRegisterer)

	// ==========================================================================
	// Dependency Injection (Clean Architecture Wiring)
	// ==========================================================================

	// Repository layer (PostgreSQL adapters).
	var eventRepo repository.EventRepository
	var subRepo repository.SubscriptionRepository
	var outboxRepo repository.OutboxRepository

	if db != nil {
		eventRepo = postgresql.NewEventRepository(db)
		subRepo = postgresql.NewSubscriptionRepository(db)
		outboxRepo = postgresql.NewOutboxRepository(db)
	}

	// Service layer.
	var ingestionSvc handler.IngestionService
	var subscriptionSvc service.SubscriptionService

	if subRepo != nil {
		subscriptionSvc = service.NewSubscriptionService(subRepo)
	}

	if eventRepo != nil && outboxRepo != nil && subRepo != nil {
		// SubRepo implements both repository.SubscriptionRepository and
		// the service.SubscriptionFinder interface for ingestion.
		if finder, ok := subRepo.(service.SubscriptionFinder); ok {
			ingestionSvc = service.NewIngestionService(eventRepo, outboxRepo, finder)
		}
	}

	// Handler layer.
	healthHandler := handler.NewHealthHandler(newDBHealthChecker(db))

	// Initialize the subscription handler if the service is available.
	var subscriptionHandler *handler.SubscriptionHandler
	if subscriptionSvc != nil {
		subscriptionHandler = handler.NewSubscriptionHandler(subscriptionSvc)
	}

	// Initialize the ingestion handler if the service is available.
	var ingestionHandler *handler.IngestionHandler
	if ingestionSvc != nil {
		ingestionHandler = handler.NewIngestionHandler(ingestionSvc)
	}

	// ==========================================================================
	// HTTP Route Registration
	// ==========================================================================

	mux := http.NewServeMux()

	// RULE-OBS-04: Liveness & readiness probes.
	mux.HandleFunc("/livez", healthHandler.HandleLivez)
	mux.HandleFunc("/readyz", healthHandler.HandleReadyz)

	// RULE-OBS-03: Expose Prometheus metrics endpoint.
	mux.Handle("/metrics", promhttp.Handler())

	// Business API endpoints (registered only when dependencies are available).
	if ingestionHandler != nil {
		mux.HandleFunc("POST /v1/events",
			handler.MetricsMiddleware("POST /v1/events", ingestionHandler.HandleIngestEvent),
		)
	}

	if subscriptionHandler != nil {
		mux.HandleFunc("POST /v1/subscriptions",
			handler.MetricsMiddleware("POST /v1/subscriptions", subscriptionHandler.HandleCreateSubscription),
		)
		mux.HandleFunc("GET /v1/subscriptions",
			handler.MetricsMiddleware("GET /v1/subscriptions", subscriptionHandler.HandleListSubscriptions),
		)
		mux.HandleFunc("GET /v1/subscriptions/{id}",
			handler.MetricsMiddleware("GET /v1/subscriptions/{id}", subscriptionHandler.HandleGetSubscription),
		)
		mux.HandleFunc("DELETE /v1/subscriptions/{id}",
			handler.MetricsMiddleware("DELETE /v1/subscriptions/{id}", subscriptionHandler.HandleDeleteSubscription),
		)
	}

	// ==========================================================================
	// HTTP Server & Graceful Shutdown
	// ==========================================================================

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           mux,
		ReadHeaderTimeout: cfg.readTimeout,
	}

	// Start server in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		logger.InfoContext(ctx, "http server listening",
			"addr", srv.Addr,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// RULE-RES-02: Deterministic lifecycle via graceful shutdown.
	shutdownCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case <-shutdownCtx.Done():
		// Signal received, begin graceful shutdown.
		logger.InfoContext(ctx, "shutting down gracefully")
	case err := <-errCh:
		logger.ErrorContext(ctx, "server error", "error", err)
		return err
	}

	shutdownTimeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownTimeoutCtx); err != nil {
		logger.ErrorContext(ctx, "graceful shutdown failed", "error", err)
		return err
	}

	logger.InfoContext(ctx, "relaybox stopped")
	return nil
}

func main() {
	// RULE-OBS-01: Structured JSON logging via slog.
	logger := log.NewJSONLogger()
	ctx := log.WithContext(context.Background(), logger)

	if err := run(ctx, logger); err != nil {
		logger.ErrorContext(ctx, "fatal startup error", "error", err)
		os.Exit(1)
	}
}
