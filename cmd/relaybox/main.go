// Package main starts the Relaybox HTTP service with liveness/readiness probes,
// event ingestion, and subscription management endpoints.
//
// Architecture wiring (Clean Architecture):
//
//	Infrastructure (memory)  →  Repository Interfaces  →  Service Layer  →  Handlers  →  Router  →  HTTP Server
//	          ↓                                                       ↑
//	   (in-memory or PostgreSQL)                          Tenant Auth Middleware
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
	"github.com/mariozul/relaybox/pkg/relaybox/infrastructure/memory"
	"github.com/mariozul/relaybox/pkg/relaybox/router"
	"github.com/mariozul/relaybox/pkg/relaybox/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// -----------------------------------------------------------------------
	// Infrastructure Layer: Repository Implementations
	// -----------------------------------------------------------------------
	// For development/demo: use in-memory repositories.
	// In production, replace with PostgreSQL-backed implementations (ST-02).
	eventRepo := memory.NewInMemoryEventRepo()
	subRepo := memory.NewInMemorySubscriptionRepo()
	outboxRepo := memory.NewInMemoryOutboxRepo()
	txManager := memory.NewInMemoryTxManager()

	// Real-time clock for production use.
	clock := time.Now

	// -----------------------------------------------------------------------
	// Application Service Layer
	// -----------------------------------------------------------------------
	ingestionSvc := service.NewEventIngestionService(eventRepo, outboxRepo, txManager, clock)
	subSvc := service.NewSubscriptionService(subRepo, clock)

	// -----------------------------------------------------------------------
	// Transport Layer: HTTP Handlers
	// -----------------------------------------------------------------------
	eventH := handler.NewEventHandler(ingestionSvc)
	subH := handler.NewSubscriptionHandler(subSvc)

	// -----------------------------------------------------------------------
	// Health Probe Handlers (FR-OBS-01, RULE-OBS-04)
	// -----------------------------------------------------------------------
	// /livez  — process is alive and serving
	// /readyz — service is ready to accept traffic (dependencies reachable)
	livezHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}

	// /readyz checks that the service is ready. With in-memory repos this is
	// always true; in production, ping the database here.
	readyzHandler := func(w http.ResponseWriter, r *http.Request) {
		// In production, replace with a real DB ping:
		//   if err := db.PingContext(r.Context()); err != nil {
		//       w.WriteHeader(http.StatusServiceUnavailable)
		//       _, _ = w.Write([]byte("not ready"))
		//       return
		//   }
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	}

	// -----------------------------------------------------------------------
	// Router Setup
	// -----------------------------------------------------------------------
	mux := router.Setup(eventH, subH, livezHandler, readyzHandler)

	// -----------------------------------------------------------------------
	// Global Middleware
	// -----------------------------------------------------------------------
	// Wrap the mux with a tenant-auth middleware that reads X-Tenant-Id header
	// and injects it into the request context (ASSUMP-01, RULE-SEC-01).
	//
	// In production, this would be JWT verification or gateway-provided identity.
	wrappedMux := tenantMiddleware(mux)

	// -----------------------------------------------------------------------
	// HTTP Server
	// -----------------------------------------------------------------------
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           wrappedMux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// -----------------------------------------------------------------------
	// Graceful Shutdown (RULE-RES-02)
	// -----------------------------------------------------------------------
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("relaybox starting",
			"version", version.Version,
			"addr", srv.Addr,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed to start", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("relaybox shutting down gracefully")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	logger.Info("relaybox stopped")
}

// tenantMiddleware extracts the X-Tenant-Id header from incoming requests and
// injects it into the request context (ASSUMP-01, RULE-SEC-01).
//
// In production, this would validate a JWT or verify a gateway-provided
// identity header. The tenant ID MUST always come from a trusted source,
// never from the request body (RULE-SEC-01).
func tenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip tenant extraction for health probes
		if r.URL.Path == "/livez" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}

		tenantID := r.Header.Get("X-Tenant-Id")
		if tenantID == "" {
			// For V1 API routes, return 403 if no tenant is provided.
			// The handlers will also check and return appropriate errors.
			slog.WarnContext(r.Context(), "missing X-Tenant-Id header",
				"path", r.URL.Path,
				"method", r.Method,
			)
			// Allow the request to proceed — handlers will return 403 if tenant
			// is missing. This enables proper error responses rather than raw
			// middleware rejection.
		}

		// Inject tenant into context (RULE-SEC-01)
		ctx := handler.WithTenant(r.Context(), tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
