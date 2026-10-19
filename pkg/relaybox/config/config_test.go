// Package config manages service configuration with environment variable loading.
package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Save and restore environment
	saveEnv(t)
	defer restoreEnv(t)

	os.Unsetenv("PORT")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("SHUTDOWN_TIMEOUT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("default Port = %q, want %q", cfg.Port, "8080")
	}
	if cfg.DatabaseURL != "postgres://localhost:5432/relaybox?sslmode=disable" {
		t.Errorf("default DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://localhost:5432/relaybox?sslmode=disable")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("default LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.ShutdownTimeoutSec != 10 {
		t.Errorf("default ShutdownTimeoutSec = %d, want %d", cfg.ShutdownTimeoutSec, 10)
	}
}

func TestLoad_EnvironmentOverrides(t *testing.T) {
	saveEnv(t)
	defer restoreEnv(t)

	os.Setenv("PORT", "9090")
	os.Setenv("DATABASE_URL", "postgres://user:pass@prod:5432/db")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("SHUTDOWN_TIMEOUT", "30")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.DatabaseURL != "postgres://user:pass@prod:5432/db" {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://user:pass@prod:5432/db")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.ShutdownTimeoutSec != 30 {
		t.Errorf("ShutdownTimeoutSec = %d, want %d", cfg.ShutdownTimeoutSec, 30)
	}
}

func TestLoad_InvalidShutdownTimeout(t *testing.T) {
	saveEnv(t)
	defer restoreEnv(t)

	os.Setenv("SHUTDOWN_TIMEOUT", "not-a-number")
	os.Setenv("DATABASE_URL", "postgres://localhost:5432/relaybox?sslmode=disable")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid SHUTDOWN_TIMEOUT, got nil")
	}
}

func TestLoad_EmptyDatabaseURL(t *testing.T) {
	saveEnv(t)
	defer restoreEnv(t)

	os.Setenv("DATABASE_URL", "")
	os.Unsetenv("DATABASE_URL") // Unset so it uses default (non-empty)

	// Actually set it to empty string explicitly
	os.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for empty DATABASE_URL, got nil")
	}
}

// Helper functions for test env management

var savedEnv = make(map[string]string)

func saveEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"PORT", "DATABASE_URL", "LOG_LEVEL", "SHUTDOWN_TIMEOUT"} {
		savedEnv[key] = os.Getenv(key)
	}
}

func restoreEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"PORT", "DATABASE_URL", "LOG_LEVEL", "SHUTDOWN_TIMEOUT"} {
		if val, ok := savedEnv[key]; ok {
			if val == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, val)
			}
		}
	}
}
