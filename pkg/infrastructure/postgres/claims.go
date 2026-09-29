package postgres

import (
	"context"
	"time"

	"github.com/mariozul/relaybox/pkg/domain"
)

func (s *Store) Claim(ctx context.Context, limit int, leaseUntil time.Time) ([]domain.Delivery, error) {
	now := s.clock.Now().UTC()
	if limit < 1 || limit > 1000 || !leaseUntil.After(now) {
		return nil, domain.ErrInvalidInput
	}
	rows, err := s.db.Query(ctx, `WITH candidates AS (
 SELECT id FROM outbox WHERE (status='pending' AND next_attempt_at<=$1) OR (status='processing' AND lease_until<=$1)
 ORDER BY next_attempt_at,id LIMIT $2 FOR UPDATE SKIP LOCKED
 ), claimed AS (
 UPDATE outbox o SET status='processing',attempts=attempts+1,lease_token=gen_random_uuid()::text,lease_until=$3,updated_at=$1,updated_by='relaybox-dispatcher'
 FROM candidates c WHERE o.id=c.id RETURNING o.*
 ) SELECT c.id,c.tenant_id,c.event_id,c.subscription_id,c.target_url,e.payload,c.trace_parent,c.trace_state,c.attempts,c.lease_token,c.lease_until,c.next_attempt_at
 FROM claimed c JOIN events e ON e.id=c.event_id AND e.tenant_id=c.tenant_id`, now, limit, leaseUntil)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	result := make([]domain.Delivery, 0, limit)
	for rows.Next() {
		var d domain.Delivery
		if err := rows.Scan(&d.ID, &d.TenantID, &d.EventID, &d.SubscriptionID, &d.TargetURL, &d.Payload, &d.TraceParent, &d.TraceState, &d.Attempts, &d.LeaseToken, &d.LeaseUntil, &d.NextAttemptAt); err != nil {
			return nil, translate(err)
		}
		d.State = domain.Processing
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, translate(err)
	}
	return result, nil
}

func (s *Store) Complete(ctx context.Context, d domain.Delivery, outcome domain.Outcome) error {
	if outcome.State != domain.Pending && outcome.State != domain.Delivered && outcome.State != domain.Deadletter {
		return domain.ErrInvalidInput
	}
	now := s.clock.Now().UTC()
	next := outcome.NextAttemptAt
	if next.IsZero() {
		next = now
	}
	result, err := s.db.Exec(ctx, `UPDATE outbox SET status=$4,next_attempt_at=$5,lease_token=NULL,lease_until=NULL,updated_at=$6,updated_by='relaybox-dispatcher' WHERE id=$1 AND tenant_id=$2 AND lease_token=$3 AND status='processing' AND lease_until>$6`, d.ID, d.TenantID, d.LeaseToken, string(outcome.State), next, now)
	if err != nil {
		return translate(err)
	}
	if result.RowsAffected() != 1 {
		return domain.ErrConflict
	}
	return nil
}
