// Package main starts the Relaybox HTTP service with liveness/readiness probes,
// event ingestion, subscription management, and delivery dispatcher.
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
	"github.com/mariozul/relaybox/internal/adapter/postgres"
	httpsrv "github.com/mariozul/relaybox/internal/transport/http"
	"github.com/mariozul/relaybox/internal/service"
	"github.com/mariozul/relaybox/internal/version"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Database pool
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://relaybox:relaybox@localhost:5432/relaybox?sslmode=disable"
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		logger.Error("failed to create db pool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Adapters
	eventStore := postgres.NewPgEventStore(pool)
	outboxStore := postgres.NewPgOutboxStore(pool)
	subscriptionStore := postgres.NewPgSubscriptionStore(pool)
	txManager := postgres.NewTxManager(pool)

	// Services
	subSvc := service.NewSubscriptionService(subscriptionStore)
	eventSvc := service.NewEventService(eventStore, outboxStore, subscriptionStore, txManager)

	// HTTP router
	router := httpsrv.NewRouter(eventSvc, subSvc, logger)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Graceful shutdown (RULE-RES-02)
	go func() {
		logger.Info("relaybox starting", "version", version.Version, "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	logger.Info("relaybox stopped")
}
