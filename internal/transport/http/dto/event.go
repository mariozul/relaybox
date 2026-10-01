package dto

type IngestRequest struct {
	EventType string `json:"event_type"`
	Payload   []byte `json:"payload"`
	DedupKey  string `json:"dedup_key"`
}

type IngestResponse struct {
	EventID   string `json:"event_id"`
	Duplicate bool   `json:"duplicate,omitempty"`
}
