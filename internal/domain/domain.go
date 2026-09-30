package domain

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid input")

type Event struct {
	ID, TenantID, EventType, DedupKey, CreatedBy string
	Payload                                      json.RawMessage
	CreatedAt                                    time.Time
}

type Subscription struct {
	ID, TenantID, EventType, TargetURL string
	CreatedAt                          time.Time
}

type Delivery struct {
	ID, TenantID, EventID, SubscriptionID, EventType, TargetURL, LeaseToken string
	Payload                                                                 json.RawMessage
	Attempts                                                                int
}

func NewEvent(eventType string, payload json.RawMessage, dedupKey string) (Event, error) {
	if strings.TrimSpace(eventType) == "" || len(eventType) > 200 || strings.TrimSpace(dedupKey) == "" || len(dedupKey) > 200 || !json.Valid(payload) {
		return Event{}, ErrInvalid
	}
	return Event{EventType: eventType, Payload: payload, DedupKey: dedupKey}, nil
}

func NewSubscription(eventType, targetURL string) (Subscription, error) {
	u, err := url.ParseRequestURI(targetURL)
	if strings.TrimSpace(eventType) == "" || len(eventType) > 200 || err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return Subscription{}, ErrInvalid
	}
	return Subscription{EventType: eventType, TargetURL: targetURL}, nil
}
