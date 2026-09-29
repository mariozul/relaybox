package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func fixture(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
	ctx := t.Context()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL required for PostgreSQL integration tests")
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.WithoutCancel(ctx)); err != nil {
			t.Error(err)
		}
	})
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.WithoutCancel(ctx)) })
	schema := pgx.Identifier{"test_" + randomSuffix(t)}.Sanitize()
	if _, err := tx.Exec(ctx, "CREATE SCHEMA "+schema+"; SET LOCAL search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	up, err := os.ReadFile("../../../migrations/202609290001_relaybox.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatal(err)
	}
	return ctx, tx
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	return "fixture"
}

func TestSchema(t *testing.T) {
	ctx, tx := fixture(t)
	for _, table := range []string{"events", "subscriptions", "outbox"} {
		var count int
		err := tx.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=$1 AND column_name IN ('created_at','created_by','updated_at','updated_by','deleted_at','deleted_by')`, table).Scan(&count)
		if err != nil || count != 6 {
			t.Fatalf("%s audit columns: %d, %v", table, count, err)
		}
	}
	for _, query := range []string{
		`INSERT INTO events(id,tenant_id,event_type,payload,dedup_key,created_by,updated_by) VALUES ('e1','a','type','{}','key','actor','actor'),('e2','b','type','{}','key','actor','actor')`,
		`INSERT INTO subscriptions(id,tenant_id,event_type,target_url,created_by,updated_by) VALUES ('s1','a','type','https://example.com','actor','actor'),('s2','b','type','https://example.com','actor','actor')`,
		`INSERT INTO outbox(id,tenant_id,event_id,subscription_id,target_url,created_by,updated_by) VALUES ('d1','a','e1','s1','https://example.com','actor','actor')`,
	} {
		if _, err := tx.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	for name, query := range map[string]string{
		"dedup":                `INSERT INTO events(id,tenant_id,event_type,payload,dedup_key,created_by,updated_by) VALUES ('e3','a','type','{}','key','actor','actor')`,
		"foreign subscription": `INSERT INTO outbox(id,tenant_id,event_id,subscription_id,target_url,created_by,updated_by) VALUES ('d2','a','e1','s2','https://example.com','actor','actor')`,
		"foreign event":        `INSERT INTO outbox(id,tenant_id,event_id,subscription_id,target_url,created_by,updated_by) VALUES ('d2','a','e2','s1','https://example.com','actor','actor')`,
		"duplicate delivery":   `INSERT INTO outbox(id,tenant_id,event_id,subscription_id,target_url,created_by,updated_by) VALUES ('d2','a','e1','s1','https://example.com','actor','actor')`,
		"invalid state":        `UPDATE outbox SET status='invalid'`,
		"partial deletion":     `UPDATE subscriptions SET deleted_at=now()`,
	} {
		t.Run(name, func(t *testing.T) {
			save, err := tx.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = save.Rollback(ctx) }()
			if _, err := save.Exec(ctx, query); err == nil {
				t.Fatal("constraint accepted invalid mutation")
			}
		})
	}
	down, err := os.ReadFile("../../../migrations/202609290001_relaybox.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	save, err := tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := save.Exec(ctx, string(down)); err == nil {
		t.Fatal("down migration erased durable records")
	}
	if err := save.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM outbox; DELETE FROM events; DELETE FROM subscriptions"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema()`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("down left tables: %d %v", count, err)
	}
}
