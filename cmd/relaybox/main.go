// Package main starts the Relaybox HTTP service with all components wired.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mariozul/relaybox/internal/api"
	"github.com/mariozul/relaybox/internal/delivery"
	"github.com/mariozul/relaybox/internal/observability"
	"github.com/mariozul/relaybox/internal/service"
	"github.com/mariozul/relaybox/internal/storage"
	"github.com/mariozul/relaybox/internal/version"
)

// realClock implements domain.Clock using time.Now.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func main() {
	logger := observability.NewLogger(slog.LevelInfo)
	metrics := observability.NewMetrics()

	// Database pool.
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://relaybox:relaybox@localhost:5432/relaybox?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := pgxpool.New(ctx, dsn)
	cancel()
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Repositories.
	eventRepo := storage.NewPostgresEventRepository(pool)
	subRepo := storage.NewPostgresSubscriptionRepository(pool)
	outboxRepo := storage.NewPostgresOutboxRepository(pool)

	// Application services.
	clock := realClock{}
	eventSvc := service.NewEventService(eventRepo, outboxRepo, subRepo, clock)
	subSvc := service.NewSubscriptionService(subRepo, clock)

	// HTTP handlers.
	eventHandler := api.NewEventHandler(eventSvc)
	subHandler := api.NewSubscriptionHandler(subSvc)

	// Health handler.
	healthHandler := api.NewHealthHandler(pool)

	// Router.
	apiRouter := api.NewRouter(eventHandler, subHandler)

	// Main mux: health + metrics + API (with tenant middleware).
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", healthHandler.Livez)
	mux.HandleFunc("/readyz", healthHandler.Readyz)
	mux.Handle("/metrics", observability.MetricsHandler())
	mux.Handle("/v1/", apiRouter)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Delivery dispatcher (bounded worker pool).
	dispatchCfg := delivery.DefaultDispatcherConfig()
	dispatchCfg.Logger = logger
	dispatchCfg.Metrics = metrics
	sender := delivery.NewHTTPSender(delivery.DefaultSenderConfig())
	dispatcher := delivery.NewDispatcher(dispatchCfg, outboxRepo, eventRepo, subRepo, sender)
	dispatcher.Start(context.Background())

	go func() {
		logger.Info("relaybox starting", "version", version.Version, "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown (RULE-RES-02).
	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-sigCtx.Done()

	logger.Info("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", "error", err)
	}
	dispatcher.Stop()
	logger.Info("relaybox stopped")
}
