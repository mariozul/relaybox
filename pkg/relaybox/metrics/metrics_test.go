package metrics_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// ============================================================================
// TC-11-UNIT: Prometheus metrics labels enforce bounded cardinality
// (FR-OBS-02, RULE-OBS-03)
// ============================================================================

func TestMetrics_Register_DoesNotPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RegisterMetrics panicked: %v", r)
		}
	}()

	registry := prometheus.NewRegistry()
	metrics.RegisterMetrics(registry)
}

func TestMetrics_HTTPRequestDuration_IsHistogram(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	metrics.RegisterMetrics(registry)

	// Record an observation so the metric appears in gathered output.
	metrics.RecordHTTPDuration("/v1/events", http.MethodPost, http.StatusOK, 100*time.Millisecond)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, family := range families {
		if family.GetName() == metrics.MetricHTTPRequestDuration {
			found = true
			if family.GetType().String() != "HISTOGRAM" {
				t.Errorf("expected type HISTOGRAM, got %s", family.GetType().String())
			}
			// Verify bounded-cardinality labels.
			metric := family.GetMetric()[0]
			for _, l := range metric.GetLabel() {
				name := l.GetName()
				if name != "path" && name != "method" && name != "status_code" {
					t.Errorf("unexpected label name %q on histogram (expected bounded labels only)", name)
				}
			}
		}
	}

	if !found {
		t.Errorf("expected %s histogram to be registered", metrics.MetricHTTPRequestDuration)
	}
}

func TestMetrics_DeliveryOutcome_IsCounter(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	metrics.RegisterMetrics(registry)

	// Record an observation so the metric appears in gathered output.
	metrics.RecordDeliveryOutcome("delivered")

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, family := range families {
		if family.GetName() == metrics.MetricDeliveryOutcomeTotal {
			found = true
			if family.GetType().String() != "COUNTER" {
				t.Errorf("expected type COUNTER, got %s", family.GetType().String())
			}
			// Verify bounded-cardinality "outcome" label.
			metric := family.GetMetric()[0]
			for _, l := range metric.GetLabel() {
				if l.GetName() == "outcome" {
					val := l.GetValue()
					if val != "delivered" && val != "failed" && val != "deadletter" {
						t.Errorf("expected outcome label to be bounded (delivered|failed|deadletter), got %q", val)
					}
				}
			}
		}
	}

	if !found {
		t.Errorf("expected %s counter to be registered", metrics.MetricDeliveryOutcomeTotal)
	}
}

func TestMetrics_NoHighCardinalityLabels(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	metrics.RegisterMetrics(registry)

	// Record some observations so metrics appear in gathered output.
	metrics.RecordHTTPDuration("/v1/events", http.MethodPost, http.StatusOK, 100*time.Millisecond)
	metrics.RecordDeliveryOutcome("delivered")

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	// High-cardinality label names that MUST NOT appear (RULE-OBS-03).
	highCardinalityLabels := map[string]bool{
		"event_id":  true,
		"tenant_id": true,
		"user_id":   true,
		"uuid":      true,
		"trace_id":  true,
		"span_id":   true,
	}

	for _, family := range families {
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if highCardinalityLabels[label.GetName()] {
					t.Errorf("metric %s has high-cardinality label %q (violates RULE-OBS-03)",
						family.GetName(), label.GetName())
				}
			}
		}
	}
}

func TestMetrics_RecordDeliveryOutcome_RecordsCounter(t *testing.T) {
	// Note: No t.Parallel() - this test observes global state.

	registry := prometheus.NewRegistry()
	metrics.RegisterMetrics(registry)

	// Record delivery outcomes on the global counter.
	metrics.RecordDeliveryOutcome("delivered")
	metrics.RecordDeliveryOutcome("delivered")
	metrics.RecordDeliveryOutcome("failed")
	metrics.RecordDeliveryOutcome("deadletter")

	// Verify the counter metric is registered.
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, family := range families {
		if family.GetName() == metrics.MetricDeliveryOutcomeTotal {
			found = true
			if len(family.GetMetric()) == 0 {
				t.Error("expected at least one metric entry for delivery outcome")
			}
		}
	}

	if !found {
		t.Errorf("expected %s to be registered", metrics.MetricDeliveryOutcomeTotal)
	}
}

func TestMetrics_RecordHTTPDuration_DoesNotPanic(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	metrics.RegisterMetrics(registry)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RecordHTTPDuration panicked: %v", r)
		}
	}()

	metrics.RecordHTTPDuration("/v1/events", http.MethodPost, http.StatusOK, 100*time.Millisecond)
}

func TestMetrics_RegisteredMetrics_AreConsistent(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	metrics.RegisterMetrics(registry)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	if len(families) == 0 {
		t.Fatal("expected at least one metric family to be registered")
	}
}
