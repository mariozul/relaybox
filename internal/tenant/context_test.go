package tenant

import (
	"context"
	"testing"
)

func TestRequire(t *testing.T) {
	t.Parallel()
	if _, err := Require(context.Background()); err == nil {
		t.Fatal("expected missing tenant error")
	}
	ctx := With(context.Background(), "tenant-a")
	if got, err := Require(ctx); err != nil || got != "tenant-a" {
		t.Fatalf("got %q, %v", got, err)
	}
}
