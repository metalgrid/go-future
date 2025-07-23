package atomic

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

var ErrFutureCancelled = errors.New("future: context cancelled before resolution")

type result[T any] struct {
	value T
	err   error
}

type Future[T any] struct {
	// Fast path: atomic pointer to result (nil = not completed)
	result atomic.Pointer[result[T]]

	// Slow path: channel for blocking when result not ready
	ch chan struct{}

	// Ensure only one completion
	completed atomic.Bool
}

var futurePool = sync.Pool{
	New: func() any {
		return &Future[any]{
			ch: make(chan struct{}, 1),
		}
	},
}

// New creates a new unresolved atomic hybrid Future.
func New[T any]() *Future[T] {
	return &Future[T]{
		ch: make(chan struct{}, 1),
	}
}

// NewFromPool creates a new unresolved Future from the pool for better performance.
func NewFromPool[T any]() *Future[T] {
	return Get[T]()
}

// Resolve completes the Future with a value.
// It must not be called after Resolve or Reject.
func (f *Future[T]) Resolve(v T) {
	if f.completed.Swap(true) {
		panic("future: Resolve or Reject already called")
	}

	// Store result atomically (fast path for future reads)
	res := &result[T]{value: v}
	f.result.Store(res)

	// Signal any waiters (slow path)
	close(f.ch)
}

// Reject completes the Future with an error.
// It must not be called after Resolve or Reject.
func (f *Future[T]) Reject(err error) {
	if f.completed.Swap(true) {
		panic("future: Resolve or Reject already called")
	}

	// Store error atomically (fast path for future reads)
	res := &result[T]{err: err}
	f.result.Store(res)

	// Signal any waiters (slow path)
	close(f.ch)
}

// IsDone returns true if the future has been resolved or rejected.
// This is the fastest possible check - just an atomic load.
func (f *Future[T]) IsDone() bool {
	return f.result.Load() != nil
}

// Wait blocks until the future is resolved, rejected, or the context is canceled.
// Uses fast path for already-completed futures, slow path for pending ones.
func (f *Future[T]) Wait(ctx context.Context) (T, error) {
	// Fast path: check if already completed
	if res := f.result.Load(); res != nil {
		if res.err != nil {
			var zero T
			return zero, res.err
		}
		return res.value, nil
	}

	// Slow path: wait for completion or context cancellation
	select {
	case <-f.ch:
		// Future completed, read result
		res := f.result.Load()
		if res.err != nil {
			var zero T
			return zero, res.err
		}
		return res.value, nil
	case <-ctx.Done():
		var zero T
		return zero, ErrFutureCancelled
	}
}

// WaitTimeout blocks until the future is resolved, rejected, or times out.
func (f *Future[T]) WaitTimeout(timeout time.Duration) (T, error) {
	// Fast path: check if already completed
	if res := f.result.Load(); res != nil {
		if res.err != nil {
			var zero T
			return zero, res.err
		}
		return res.value, nil
	}

	// Slow path: wait with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return f.Wait(ctx)
}

// Reset clears the future state for reuse. Must be called before returning to pool.
func (f *Future[T]) Reset() {
	f.result.Store(nil)
	f.completed.Store(false)

	// Recreate channel if it was closed
	select {
	case <-f.ch:
		// Channel was closed, need new one
		f.ch = make(chan struct{}, 1)
	default:
		// Channel is still open, keep it
	}
}

// Get retrieves a Future from the pool, ready for use.
func Get[T any]() *Future[T] {
	f := futurePool.Get().(*Future[any])
	return (*Future[T])(unsafe.Pointer(f))
}

// Put returns a Future to the pool for reuse. The Future is reset before pooling.
func Put[T any](f *Future[T]) {
	if f == nil {
		return
	}
	f.Reset()
	futurePool.Put((*Future[any])(unsafe.Pointer(f)))
}
