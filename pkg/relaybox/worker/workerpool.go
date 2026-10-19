// Package worker provides a bounded worker pool for managing concurrent task
// execution with graceful shutdown support (FR-DEL-03, RULE-RES-02).
package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// ErrQueueFull is returned when the worker pool's task queue is at capacity.
var ErrQueueFull = errors.New("worker pool queue is full")

// TaskFunc represents a unit of work to be executed by the worker pool.
type TaskFunc func(ctx context.Context)

// WorkerPool defines the contract for a bounded worker pool.
type WorkerPool interface {
	// Submit enqueues a task for execution. Returns ErrQueueFull if the
	// task queue is at capacity. Panics if called after Stop().
	Submit(task TaskFunc) error

	// Wait blocks until all queued tasks have been processed and all
	// workers have terminated.
	Wait()

	// Stop terminates the worker pool gracefully. After Stop(), Submit
	// will panic. Workers finish their current task before exiting.
	// Call Wait() after Stop() to ensure all tasks complete.
	Stop()
}

// boundedWorkerPool implements WorkerPool.
type boundedWorkerPool struct {
	ctx       context.Context
	cancel    context.CancelFunc
	taskQueue chan TaskFunc
	quit      chan struct{}
	wg        sync.WaitGroup
	stopped   atomic.Bool
}

// NewWorkerPool creates a new bounded worker pool.
func NewWorkerPool(ctx context.Context, numWorkers int, queueSize int) WorkerPool {
	poolCtx, cancel := context.WithCancel(ctx)

	pool := &boundedWorkerPool{
		ctx:       poolCtx,
		cancel:    cancel,
		taskQueue: make(chan TaskFunc, queueSize),
		quit:      make(chan struct{}),
	}

	pool.wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go pool.worker()
	}

	return pool
}

// worker reads tasks from the queue and executes them.
func (p *boundedWorkerPool) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.quit:
			// Drain remaining tasks from queue before exiting
			for {
				select {
				case task, ok := <-p.taskQueue:
					if !ok {
						return
					}
					p.executeTask(task)
				default:
					return
				}
			}
		case task, ok := <-p.taskQueue:
			if !ok {
				return
			}
			p.executeTask(task)
		}
	}
}

// executeTask runs a single task function, recovering panics.
func (p *boundedWorkerPool) executeTask(task TaskFunc) {
	defer func() { _ = recover() }()
	task(p.ctx)
}

// Submit enqueues a task. Returns ErrQueueFull if queue is full.
// Panics if called after Stop().
func (p *boundedWorkerPool) Submit(task TaskFunc) error {
	if p.stopped.Load() {
		panic("worker pool: Submit called after Stop")
	}

	select {
	case p.taskQueue <- task:
		return nil
	default:
		return ErrQueueFull
	}
}

// Wait blocks until all workers have exited.
func (p *boundedWorkerPool) Wait() {
	p.wg.Wait()
}

// Stop signals shutdown. After returning, further Submit calls panic.
// Workers finish their current task and drain remaining queue before exiting.
// Call Wait() after Stop() to ensure all tasks complete.
func (p *boundedWorkerPool) Stop() {
	p.stopped.Store(true)
	close(p.quit)
}

// compile-time interface check.
var _ WorkerPool = (*boundedWorkerPool)(nil)
