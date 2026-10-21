// Package dto defines data transfer objects for the relaybox API layer.
package dto

import "encoding/json"

// IngestEventRequest represents the incoming JSON request for event ingestion
// per FR-ING-01: { event_type, payload, dedup_key }.
type IngestEventRequest struct {
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	DedupKey  string          `json:"dedup_key"`
}
