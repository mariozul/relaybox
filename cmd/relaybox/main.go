// Package main starts the Relaybox HTTP service with liveness/readiness probes.
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

	"github.com/mariozul/relaybox/internal/adapter"
	"github.com/mariozul/relaybox/internal/config"
	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/service"
	httptransport "github.com/mariozul/relaybox/internal/transport/http"
	"github.com/mariozul/relaybox/internal/version"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	// Database pool.
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to create db pool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Repositories.
	eventRepo := adapter.NewPGEventRepo(pool)
	subRepo := adapter.NewPGSubscriptionRepo(pool)
	outboxRepo := adapter.NewPGOutboxRepo(pool)
	txManager := adapter.NewPGTxManager(pool)

	// Services.
	clock := domain.RealClock{}
	ingestSvc := service.NewIngestService(eventRepo, subRepo, outboxRepo, txManager, clock)
	subSvc := service.NewSubscriptionService(subRepo)

	// HTTP forwarder.
	forwarder := adapter.NewHTTPForwarder(nil, 30*time.Second)

	// Dispatcher.
	dispCfg := service.DefaultDispatcherConfig()
	dispatcher := service.NewDispatcher(dispCfg, outboxRepo, eventRepo, forwarder, clock, logger)
	go dispatcher.Start(context.Background())

	// HTTP handlers.
	ingestHandler := httptransport.NewIngestHandler(ingestSvc)
	subHandler := httptransport.NewSubscriptionHandler(subSvc)
	healthHandler := httptransport.NewHealthHandler(pool)

	// Routes.
	mux := http.NewServeMux()
	httptransport.RegisterRoutes(mux, ingestHandler, subHandler, healthHandler)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("relaybox starting", "version", version.Version, "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown (RULE-RES-02).
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
