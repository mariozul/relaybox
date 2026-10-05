package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestMetricsCounterAndHistogram(t *testing.T) {
	t.Parallel()
	m := NewMetrics("relaybox")
	c := m.Counter("ingest", "events")
	c.WithLabelValues("success").Inc()
	c.WithLabelValues("success").Inc()
	c.WithLabelValues("failure").Inc()
	h := m.Histogram("delivery", "dispatch")
	h.WithLabelValues("success").Observe(0.05)
	h.WithLabelValues("failure").Observe(1.5)

	mfs, err := m.Registry().Gather()
	if err != nil {
		t.Fatal(err)
	}
	cOK, hOK := false, false
	for _, mf := range mfs {
		switch mf.GetName() {
		case "app_relaybox_ingest_events_total":
			for _, mt := range mf.GetMetric() {
				if labelPairs(mt.GetLabel())["status"] == "success" && mt.GetCounter().GetValue() != 2 {
					t.Error("counter mismatch")
				}
			}
			cOK = true
		case "app_relaybox_delivery_dispatch_duration_seconds":
			hOK = true
		}
	}
	if !cOK || !hOK {
		t.Fatal("missing metrics")
	}
}

func TestMetricsLabelCardinalityBounded(t *testing.T) {
	t.Parallel()
	m := NewMetrics("relaybox")
	m.Counter("sub", "create").WithLabelValues("success").Inc()
	mfs, _ := m.Registry().Gather()
	for _, mf := range mfs {
		if mf.GetName() == "app_relaybox_sub_create_total" {
			for _, mt := range mf.GetMetric() {
				lbs := mt.GetLabel()
				if len(lbs) != 1 || lbs[0].GetName() != "status" {
					t.Errorf("label invariant violated: %v", lbs)
				}
			}
		}
	}
}

func TestMetricsHTTPHandler(t *testing.T) {
	t.Parallel()
	m := NewMetrics("relaybox")
	m.Counter("ingest", "events").WithLabelValues("success").Inc()
	rec := httptest.NewRecorder()
	m.HTTPHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "app_relaybox_ingest_events_total") {
		t.Error("metric name missing")
	}
}

func TestMetricsConcurrencySafe(t *testing.T) {
	m := NewMetrics("relaybox")
	c := m.Counter("delivery", "retry")
	h := m.Histogram("delivery", "retry")
	var wg sync.WaitGroup
	const n = 50
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.WithLabelValues("success").Inc()
				h.WithLabelValues("success").Observe(0.01)
			}
		}()
	}
	wg.Wait()
	mfs, _ := m.Registry().Gather()
	for _, mf := range mfs {
		if mf.GetName() == "app_relaybox_delivery_retry_total" {
			for _, mt := range mf.GetMetric() {
				if labelPairs(mt.GetLabel())["status"] == "success" &&
					mt.GetCounter().GetValue() != float64(n*100) {
					t.Error("race: counter mismatch")
				}
			}
		}
	}
}

func TestHealthProbes(t *testing.T) {
	t.Run("livez", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		LivezHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/livez", nil))
		resp := rec.Result()
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != 200 || mustBody(resp) != "ok\n" {
			t.Error("livez broken")
		}
	})
	t.Run("readyz-ok", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		ReadyzHandler(map[string]HealthChecker{
			"db": func(ctx context.Context) error { return nil },
		}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		resp := rec.Result()
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != 200 {
			t.Error("healthy readyz")
		}
	})
	t.Run("readyz-503", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		ReadyzHandler(map[string]HealthChecker{
			"db": func(ctx context.Context) error { return errors.New("down") },
		}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		resp := rec.Result()
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != 503 || !strings.Contains(mustBody(resp), "not ready") {
			t.Error("unhealthy readyz")
		}
	})
	t.Run("readyz-ctx", func(t *testing.T) {
		t.Parallel()
		var got context.Context
		rec := httptest.NewRecorder()
		ReadyzHandler(map[string]HealthChecker{
			"x": func(c context.Context) error { got = c; return nil },
		}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		_ = rec.Result().Body.Close()
		if got == nil {
			t.Error("ctx not propagated")
		}
	})
	t.Run("readyz-nil", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		ReadyzHandler(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		resp := rec.Result()
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != 200 {
			t.Error("nil checkers")
		}
	})
}

func TestStructuredLogging(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		NewLogger(&buf).Info("hello", "k", "v")
		var e map[string]interface{}
		_ = json.Unmarshal(buf.Bytes(), &e)
		if e["msg"] != "hello" || e["level"] != "INFO" || e["time"] == nil {
			t.Error("json invariant broken")
		}
	})
	tid, _ := trace.TraceIDFromHex("00000000000000010000000000000002")
	sid, _ := trace.SpanIDFromHex("0000000000000003")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled,
	})
	tctx := trace.ContextWithSpanContext(context.Background(), sc)

	t.Run("infoctx", func(t *testing.T) {
		var buf bytes.Buffer
		NewLogger(&buf).InfoContext(tctx, "traced")
		var e map[string]interface{}
		_ = json.Unmarshal(buf.Bytes(), &e)
		if e["trace_id"] == "" || e["span_id"] == "" {
			t.Error("trace attributes missing")
		}
	})
	t.Run("errorctx", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		NewLogger(&buf).ErrorContext(context.Background(), "boom", "err", errors.New("fail"))
		var e map[string]interface{}
		_ = json.Unmarshal(buf.Bytes(), &e)
		if e["level"] != "ERROR" {
			t.Error("level mismatch")
		}
	})
	t.Run("withlevel", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		NewLogger(&buf, WithLevel(slog.LevelDebug)).Debug("dbg")
		var e map[string]interface{}
		_ = json.Unmarshal(buf.Bytes(), &e)
		if e["level"] != "DEBUG" {
			t.Error("level option broken")
		}
	})
}

func TestOTelTraceContext(t *testing.T) {
	t.Run("extract-empty", func(t *testing.T) {
		t.Parallel()
		tid, sid := ExtractTraceContext(context.Background())
		if tid != "" || sid != "" {
			t.Error("want empty from bg ctx")
		}
	})
	tid, _ := trace.TraceIDFromHex("00000000000000010000000000000002")
	sid, _ := trace.SpanIDFromHex("0000000000000003")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled,
	})
	vctx := trace.ContextWithSpanContext(context.Background(), sc)

	t.Run("extract-valid", func(t *testing.T) {
		tr, sp := ExtractTraceContext(vctx)
		if tr == "" || sp == "" {
			t.Error("want non-empty from valid span")
		}
	})
	t.Run("inject", func(t *testing.T) {
		orig := otel.GetTextMapPropagator()
		otel.SetTextMapPropagator(propagation.TraceContext{})
		defer otel.SetTextMapPropagator(orig)
		h := http.Header{}
		InjectTraceContext(vctx, h)
		if h.Get("traceparent") == "" {
			t.Error("traceparent not injected")
		}
		InjectTraceContext(context.Background(), http.Header{})
	})
}

func labelPairs(labels []*dto.LabelPair) map[string]string {
	m := make(map[string]string, len(labels))
	for _, lp := range labels {
		m[lp.GetName()] = lp.GetValue()
	}
	return m
}

func mustBody(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}
