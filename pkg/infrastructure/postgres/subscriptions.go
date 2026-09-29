package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mariozul/relaybox/pkg/domain"
)

type DB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type Store struct {
	db    DB
	clock domain.Clock
}

func New(db DB, clock domain.Clock) *Store { return &Store{db: db, clock: clock} }

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", domain.ErrUnavailable
	}
	return hex.EncodeToString(b[:]), nil
}

func (s *Store) CreateSubscription(ctx context.Context, identity domain.Identity, input domain.SubscriptionInput) (domain.Subscription, error) {
	if err := identity.Validate(); err != nil {
		return domain.Subscription{}, err
	}
	u, err := url.Parse(input.TargetURL)
	if strings.TrimSpace(input.EventType) == "" || err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return domain.Subscription{}, domain.ErrInvalidInput
	}
	id, err := newID()
	if err != nil {
		return domain.Subscription{}, err
	}
	now := s.clock.Now().UTC()
	sub := domain.Subscription{ID: id, TenantID: identity.TenantID, SubscriptionInput: input, Audit: domain.Audit{CreatedAt: now, UpdatedAt: now, CreatedBy: identity.ActorID, UpdatedBy: identity.ActorID}}
	_, err = s.db.Exec(ctx, `INSERT INTO subscriptions (id,tenant_id,event_type,target_url,created_at,updated_at,created_by,updated_by) VALUES ($1,$2,$3,$4,$5,$5,$6,$6)`, id, identity.TenantID, input.EventType, input.TargetURL, now, identity.ActorID)
	if err != nil {
		return domain.Subscription{}, translate(err)
	}
	return sub, nil
}

func (s *Store) ListSubscriptions(ctx context.Context, identity domain.Identity) ([]domain.Subscription, error) {
	if err := identity.Validate(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT id,tenant_id,event_type,target_url,created_at,updated_at,created_by,updated_by FROM subscriptions WHERE tenant_id=$1 AND deleted_at IS NULL ORDER BY id`, identity.TenantID)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	result := make([]domain.Subscription, 0)
	for rows.Next() {
		var sub domain.Subscription
		if err := rows.Scan(&sub.ID, &sub.TenantID, &sub.EventType, &sub.TargetURL, &sub.Audit.CreatedAt, &sub.Audit.UpdatedAt, &sub.Audit.CreatedBy, &sub.Audit.UpdatedBy); err != nil {
			return nil, translate(err)
		}
		result = append(result, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, translate(err)
	}
	return result, nil
}

func (s *Store) DeleteSubscription(ctx context.Context, identity domain.Identity, id string) error {
	if err := identity.Validate(); err != nil {
		return err
	}
	result, err := s.db.Exec(ctx, `UPDATE subscriptions SET deleted_at=$3,updated_at=$3,deleted_by=$4,updated_by=$4 WHERE tenant_id=$1 AND id=$2 AND deleted_at IS NULL`, identity.TenantID, id, s.clock.Now().UTC(), identity.ActorID)
	if err != nil {
		return translate(err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func translate(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		if pgerr.Code == "23505" {
			return domain.ErrConflict
		}
		if pgerr.Code == "23503" || pgerr.Code == "23514" || pgerr.Code == "23502" {
			return domain.ErrInvalidInput
		}
	}
	return domain.ErrUnavailable
}
