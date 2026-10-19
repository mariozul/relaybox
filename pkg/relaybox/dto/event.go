// Package dto defines data transfer objects for the relaybox API layer.
package dto

// IngestEventRequest represents the incoming JSON request for event ingestion
// per FR-ING-01: { event_type, payload, dedup_key }.
type IngestEventRequest struct {
	EventType string `json:"event_type" binding:"required"`
	Payload   []byte `json:"payload" binding:"required"`
	DedupKey  string `json:"dedup_key" binding:"required"`
}
