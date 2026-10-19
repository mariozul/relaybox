package worker_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/worker"
)

func TestWorkerPool(t *testing.T) {
	t.Parallel()

	t.Run("processes all submitted tasks", func(t *testing.T) {
		t.Parallel()
		pool := worker.NewWorkerPool(context.Background(), 2, 100)
		var counter int32
		for i := 0; i < 20; i++ {
			if err := pool.Submit(func(ctx context.Context) { atomic.AddInt32(&counter, 1) }); err != nil {
				t.Fatalf("submit %d: %v", i, err)
			}
		}
		pool.Stop()
		pool.Wait()
		if got := atomic.LoadInt32(&counter); got != 20 {
			t.Errorf("expected 20, got %d", got)
		}
	})

	t.Run("bounds concurrency to pool size", func(t *testing.T) {
		t.Parallel()
		pool := worker.NewWorkerPool(context.Background(), 3, 100)
		var active, maxActive int32
		done := make(chan int32, 20)
		for i := 0; i < 10; i++ {
			pool.Submit(func(ctx context.Context) {
				v := atomic.AddInt32(&active, 1)
				done <- v
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt32(&active, -1)
			})
		}
		pool.Stop()
		pool.Wait()
		close(done)
		for v := range done {
			if v > maxActive {
				maxActive = v
			}
		}
		if maxActive > 3 {
			t.Errorf("expected max <= 3, got %d", maxActive)
		}
	})

	t.Run("panics on submit after stop", func(t *testing.T) {
		t.Parallel()
		pool := worker.NewWorkerPool(context.Background(), 1, 1)
		pool.Stop()
		pool.Wait()
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()
		pool.Submit(func(ctx context.Context) {})
	})

	t.Run("returns error when queue full", func(t *testing.T) {
		t.Parallel()
		pool := worker.NewWorkerPool(context.Background(), 0, 2)
		_ = pool.Submit(func(ctx context.Context) {})
		_ = pool.Submit(func(ctx context.Context) {})
		if err := pool.Submit(func(ctx context.Context) {}); err != worker.ErrQueueFull {
			t.Fatalf("expected ErrQueueFull, got %v", err)
		}
		pool.Stop()
		pool.Wait()
	})
}
