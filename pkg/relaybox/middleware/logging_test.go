// Package middleware implements HTTP middleware for logging and metrics.
package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// logMu serialises logging tests that replace the default slog logger.
var logMu sync.Mutex

// ---------------------------------------------------------------------------
// TC-OBS-02-UNIT: Structured JSON logging (RULE-OBS-01)
// ---------------------------------------------------------------------------

func TestStructuredLogger_EmitsJSON(t *testing.T) {
	logMu.Lock()
	defer logMu.Unlock()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	handler := StructuredLogger()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	// Verify JSON log was emitted
	line := buf.String()
	if line == "" {
		t.Fatal("expected JSON log output, got empty")
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
		t.Fatalf("expected valid JSON log, got: %s (err: %v)", line, err)
	}

	// Must have standard fields (RULE-OBS-01)
	expectedFields := []string{"method", "path", "status", "latency"}
	for _, field := range expectedFields {
		if _, ok := logEntry[field]; !ok {
			t.Errorf("expected log field %q in JSON output", field)
		}
	}

	if logEntry["method"] != "GET" {
		t.Errorf("expected method GET, got %v", logEntry["method"])
	}
	if logEntry["path"] != "/livez" {
		t.Errorf("expected path /livez, got %v", logEntry["path"])
	}
	if status, ok := logEntry["status"].(float64); !ok || int(status) != 200 {
		t.Errorf("expected status 200, got %v", logEntry["status"])
	}
}

func TestStructuredLogger_PostRequest(t *testing.T) {
	logMu.Lock()
	defer logMu.Unlock()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	handler := StructuredLogger()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader([]byte(`{"key":"val"}`)))
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(buf.String()), &logEntry); err != nil {
		t.Fatalf("expected valid JSON: %s (err: %v)", buf.String(), err)
	}

	if logEntry["method"] != "POST" {
		t.Errorf("expected method POST, got %v", logEntry["method"])
	}
	if logEntry["path"] != "/v1/events" {
		t.Errorf("expected path /v1/events, got %v", logEntry["path"])
	}
	if status, ok := logEntry["status"].(float64); !ok || int(status) != 201 {
		t.Errorf("expected status 201, got %v", logEntry["status"])
	}
}

func TestStructuredLogger_NoPanicOnMissingFields(t *testing.T) {
	logMu.Lock()
	defer logMu.Unlock()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	handler := StructuredLogger()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/v1/subscriptions/123", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	_ = w.Result().Body.Close()

	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(buf.String()), &logEntry); err != nil {
		t.Fatalf("expected valid JSON: %s (err: %v)", buf.String(), err)
	}

	// Should not panic even on error response
	if status, ok := logEntry["status"].(float64); !ok || int(status) != 500 {
		t.Errorf("expected status 500, got %v", logEntry["status"])
	}
}
