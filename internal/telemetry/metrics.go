package telemetry

import "sync"

const (
	OutcomeDelivered  = "delivered"
	OutcomeFailed     = "failed"
	OutcomeDeadLetter = "dead_letter"
)

type Metrics struct {
	mu              sync.Mutex
	IngestTotal     int64
	DeliveryTotal   int64
	DeliveryOutcome map[string]int64
}

var validOutcomes = map[string]bool{
	OutcomeDelivered: true, OutcomeFailed: true, OutcomeDeadLetter: true,
}

func NewMetrics() *Metrics {
	return &Metrics{
		DeliveryOutcome: map[string]int64{
			OutcomeDelivered:  0,
			OutcomeFailed:     0,
			OutcomeDeadLetter: 0,
		},
	}
}

func (m *Metrics) RecordIngest() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.IngestTotal++
}

func (m *Metrics) RecordDelivery(label string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !validOutcomes[label] {
		return // Guard against unbounded cardinality (RULE-OBS-03).
	}
	m.DeliveryTotal++
	m.DeliveryOutcome[label]++
}

func (m *Metrics) Snapshot() (ingest, delivery int64, outcomes map[string]int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]int64{}
	for k, v := range m.DeliveryOutcome {
		out[k] = v
	}
	return m.IngestTotal, m.DeliveryTotal, out
}
