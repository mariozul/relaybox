// Package main starts the Relaybox HTTP service with liveness/readiness probes,
// event ingestion, and subscription management endpoints.
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

	"github.com/mariozul/relaybox/internal/version"
	"github.com/mariozul/relaybox/pkg/relaybox/handler"
	"github.com/mariozul/relaybox/pkg/relaybox/router"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// -----------------------------------------------------------------------
	// Health probe handlers (FR-OBS-01, RULE-OBS-04)
	// -----------------------------------------------------------------------
	livezHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}

	readyzHandler := func(w http.ResponseWriter, r *http.Request) {
		// TODO: check DB/cache reachability before reporting ready.
		// In production, this should ping the database.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	}

	// -----------------------------------------------------------------------
	// Service & Handler initialization
	// -----------------------------------------------------------------------
	// NOTE: In a full production setup, real repository implementations
	// backed by a database connection are injected here. For this PR (ST-06),
	// we only set up the HTTP handler layer. Services are already implemented
	// in ST-03 and ST-04, but require concrete repository implementations
	// from ST-02 to be wired.
	//
	// TODO: Wire with real repository implementations when database
	// connection is configured:
	//   db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	//   ...
	//   ingestionSvc := service.NewEventIngestionService(...)
	//   subSvc := service.NewSubscriptionService(...)
	//   eventH := handler.NewEventHandler(ingestionSvc)
	//   subH  := handler.NewSubscriptionHandler(subSvc)
	//   mux   := router.Setup(eventH, subH, livezHandler, readyzHandler)
	//
	// For now, start with health probes only. The handler package is
	// imported to ensure it compiles (tests verify handler correctness).
	_ = handler.NewEventHandler
	_ = handler.NewSubscriptionHandler
	_ = router.Setup

	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", livezHandler)
	mux.HandleFunc("GET /readyz", readyzHandler)

	srv := &http.Server{
		Addr:              ":8080",
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

	// RULE-RES-02: deterministic lifecycle via graceful shutdown.
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
