// Package config manages service configuration loaded from environment variables.
//
// Config provides a centralized, validated configuration struct that is populated
// from environment variables at startup. Default values are defined for local
// development, and production configurations override them via environment.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the runtime configuration for the Relaybox service.
type Config struct {
	// Port is the HTTP listen address (default: "8080").
	Port string

	// DatabaseURL is the PostgreSQL connection string.
	DatabaseURL string

	// LogLevel controls the minimum log level (default: "info").
	// Valid values: "debug", "info", "warn", "error".
	LogLevel string

	// ShutdownTimeoutSec is the graceful shutdown timeout in seconds (default: 10).
	ShutdownTimeoutSec int
}

// Load reads configuration from environment variables and returns a validated Config.
//
// Environment variables:
//   - PORT:              HTTP listen port (default: "8080")
//   - DATABASE_URL:      PostgreSQL connection string (required)
//   - LOG_LEVEL:         Log level (default: "info")
//   - SHUTDOWN_TIMEOUT:  Graceful shutdown timeout in seconds (default: 10)
func Load() (*Config, error) {
	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://localhost:5432/relaybox?sslmode=disable")
	logLevel := getEnv("LOG_LEVEL", "info")
	shutdownTimeoutStr := getEnv("SHUTDOWN_TIMEOUT", "10")

	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required but not set")
	}

	shutdownTimeout, err := strconv.Atoi(shutdownTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SHUTDOWN_TIMEOUT %q: must be a positive integer", shutdownTimeoutStr)
	}
	if shutdownTimeout <= 0 {
		return nil, fmt.Errorf("SHUTDOWN_TIMEOUT must be positive, got %d", shutdownTimeout)
	}

	return &Config{
		Port:               port,
		DatabaseURL:        dbURL,
		LogLevel:           logLevel,
		ShutdownTimeoutSec: shutdownTimeout,
	}, nil
}

// getEnv returns the value of an environment variable or a default value if not set.
// Uses os.LookupEnv to distinguish between unset and empty values.
func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}
