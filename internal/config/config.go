// Package config provides typed configuration for the Relaybox service.
package config

import "os"

// Config holds all service configuration.
type Config struct {
	DatabaseURL string
	HTTPAddr    string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() Config {
	cfg := Config{
		DatabaseURL: "postgres://relaybox:relaybox@localhost:5432/relaybox?sslmode=disable",
		HTTPAddr:    ":8080",
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	return cfg
}
