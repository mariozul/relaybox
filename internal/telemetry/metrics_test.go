package telemetry

import (
	"testing"
)

func TestMetricsBoundedLabels(t *testing.T) {
	t.Parallel()
	m := NewMetrics()

	// Valid labels only.
	m.RecordDelivery(OutcomeDelivered)
	m.RecordDelivery(OutcomeFailed)
	m.RecordDelivery(OutcomeDeadLetter)

	// Unbounded label should be ignored.
	m.RecordDelivery("tenant-123") // would break cardinality budget, should be ignored

	ingest, delivery, outcomes := m.Snapshot()
	if ingest != 0 {
		t.Errorf("ingest=%d", ingest)
	}
	if delivery != 3 {
		t.Errorf("delivery=%d", delivery)
	}
	if outcomes[OutcomeDelivered] != 1 {
		t.Errorf("delivered=%d", outcomes[OutcomeDelivered])
	}
	if outcomes[OutcomeFailed] != 1 {
		t.Errorf("failed=%d", outcomes[OutcomeFailed])
	}
	if outcomes[OutcomeDeadLetter] != 1 {
		t.Errorf("dead_letter=%d", outcomes[OutcomeDeadLetter])
	}
	// tenant-123 should NOT be recorded.
	if outcomes["tenant-123"] != 0 {
		t.Error("unbounded label should not be recorded")
	}
}

func TestMetricsIngestCounter(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	m.RecordIngest()
	m.RecordIngest()
	m.RecordIngest()
	ingest, _, _ := m.Snapshot()
	if ingest != 3 {
		t.Errorf("ingest=%d", ingest)
	}
}

func TestMetricsConcurrencySafe(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				m.RecordIngest()
				m.RecordDelivery(OutcomeDelivered)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	ingest, delivery, _ := m.Snapshot()
	if ingest != 1000 {
		t.Errorf("ingest=%d", ingest)
	}
	if delivery != 1000 {
		t.Errorf("delivery=%d", delivery)
	}
}
