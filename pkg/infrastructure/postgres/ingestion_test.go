package postgres

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/mariozul/relaybox/pkg/domain"
)

func TestIngestion(t *testing.T) {
	ctx, tx := fixture(t)
	store := New(tx, fixedClock{})
	owner := domain.Identity{TenantID: "a", ActorID: "actor"}
	other := domain.Identity{TenantID: "b", ActorID: "other"}
	for _, identity := range []domain.Identity{owner, other} {
		if _, err := store.CreateSubscription(ctx, identity, domain.SubscriptionInput{EventType: "order", TargetURL: "https://example.com"}); err != nil {
			t.Fatal(err)
		}
	}
	input := domain.EventInput{EventType: "order", DedupKey: "key", Payload: []byte(`{"x":1}`)}
	event, err := store.Ingest(ctx, owner, input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := store.Ingest(ctx, owner, input)
	if err != nil || again.ID != event.ID {
		t.Fatalf("duplicate: %v %v", again, err)
	}
	if _, err := store.Ingest(ctx, other, input); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM outbox WHERE tenant_id='a'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("fanout: %d %v", count, err)
	}
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM events").Scan(&count); err != nil || count != 2 {
		t.Fatalf("events: %d %v", count, err)
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE outbox ADD CONSTRAINT reject_new CHECK (tenant_id <> 'c')`); err != nil {
		t.Fatal(err)
	}
	failing := domain.Identity{TenantID: "c", ActorID: "actor"}
	if _, err := store.CreateSubscription(ctx, failing, domain.SubscriptionInput{EventType: "order", TargetURL: "https://example.com"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Ingest(ctx, failing, input); err == nil {
		t.Fatal("expected outbox failure")
	}
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM events WHERE tenant_id='c'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial commit: %d %v", count, err)
	}
}

func TestConcurrentIngestion(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL required")
	}
	ctx := t.Context()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	id, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	schema := pgx.Identifier{"concurrent_" + id}.Sanitize()
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+schema+"; SET search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = conn.Exec(context.WithoutCancel(ctx), "DROP SCHEMA "+schema+" CASCADE") }()
	up, err := os.ReadFile("../../../migrations/202609290001_relaybox.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, string(up)); err != nil {
		t.Fatal(err)
	}
	owner := domain.Identity{TenantID: "tenant", ActorID: "actor"}
	if _, err := New(conn, fixedClock{}).CreateSubscription(ctx, owner, domain.SubscriptionInput{EventType: "order", TargetURL: "https://example.com"}); err != nil {
		t.Fatal(err)
	}
	const workers = 8
	ids := make(chan string, workers)
	failures := make(chan error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := pgx.Connect(ctx, dsn)
			if err != nil {
				failures <- err
				return
			}
			defer func() { _ = c.Close(context.WithoutCancel(ctx)) }()
			if _, err := c.Exec(ctx, "SET search_path TO "+schema); err != nil {
				failures <- err
				return
			}
			<-start
			e, err := New(c, fixedClock{}).Ingest(ctx, owner, domain.EventInput{EventType: "order", DedupKey: "same", Payload: []byte(`{}`)})
			if err != nil {
				failures <- err
				return
			}
			ids <- e.ID
		}()
	}
	close(start)
	wg.Wait()
	close(ids)
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	original := ""
	for id := range ids {
		if original == "" {
			original = id
		}
		if original != id {
			t.Fatal("different IDs for same key")
		}
	}
	for _, table := range []string{"events", "outbox"} {
		var count int
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s count=%d err=%v", table, count, err)
		}
	}
}
