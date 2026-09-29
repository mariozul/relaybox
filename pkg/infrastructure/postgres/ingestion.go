package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mariozul/relaybox/pkg/domain"
)

type beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

func (s *Store) Ingest(ctx context.Context, identity domain.Identity, input domain.EventInput) (domain.Event, error) {
	if err := identity.Validate(); err != nil {
		return domain.Event{}, err
	}
	if strings.TrimSpace(input.EventType) == "" || strings.TrimSpace(input.DedupKey) == "" || !json.Valid(input.Payload) {
		return domain.Event{}, domain.ErrInvalidInput
	}
	db, ok := s.db.(beginner)
	if !ok {
		return domain.Event{}, domain.ErrUnavailable
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return domain.Event{}, translate(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	id, err := newID()
	if err != nil {
		return domain.Event{}, err
	}
	now := s.clock.Now().UTC()
	result, err := tx.Exec(ctx, `INSERT INTO events(id,tenant_id,event_type,payload,dedup_key,created_at,updated_at,created_by,updated_by) VALUES ($1,$2,$3,$4,$5,$6,$6,$7,$7) ON CONFLICT (tenant_id,dedup_key) DO NOTHING`, id, identity.TenantID, input.EventType, input.Payload, input.DedupKey, now, identity.ActorID)
	if err != nil {
		return domain.Event{}, translate(err)
	}
	var event domain.Event
	err = tx.QueryRow(ctx, `SELECT id,tenant_id,event_type,payload,dedup_key,created_at,updated_at,created_by,updated_by FROM events WHERE tenant_id=$1 AND dedup_key=$2`, identity.TenantID, input.DedupKey).Scan(&event.ID, &event.TenantID, &event.EventType, &event.Payload, &event.DedupKey, &event.Audit.CreatedAt, &event.Audit.UpdatedAt, &event.Audit.CreatedBy, &event.Audit.UpdatedBy)
	if err != nil {
		return domain.Event{}, translate(err)
	}
	if result.RowsAffected() == 1 {
		_, err = tx.Exec(ctx, `INSERT INTO outbox(id,tenant_id,event_id,subscription_id,target_url,created_at,updated_at,created_by,updated_by,next_attempt_at) SELECT gen_random_uuid()::text,tenant_id,$1,id,target_url,$4,$4,$5,$5,$4 FROM subscriptions WHERE tenant_id=$2 AND event_type=$3 AND deleted_at IS NULL`, event.ID, identity.TenantID, event.EventType, now, identity.ActorID)
		if err != nil {
			return domain.Event{}, translate(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Event{}, translate(err)
	}
	return event, nil
}
