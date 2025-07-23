package ch

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
	ch   chan result[T]
	done atomic.Bool
}

var futurePool = sync.Pool{
	New: func() interface{} {
		return &Future[any]{
			ch: make(chan result[any], 1),
		}
	},
}

// New creates a new unresolved channel-based Future.
func New[T any]() *Future[T] {
	return &Future[T]{
		ch: make(chan result[T], 1),
	}
}

// NewFromPool creates a new unresolved Future from the pool for better performance.
func NewFromPool[T any]() *Future[T] {
	return Get[T]()
}

// Resolve completes the Future with a value.
// It must not be called after Resolve or Reject.
func (f *Future[T]) Resolve(v T) {
	if f.done.Swap(true) {
		panic("future: Resolve or Reject already called")
	}
	f.ch <- result[T]{value: v}
}

// Reject completes the Future with an error.
// It must not be called after Resolve or Reject.
func (f *Future[T]) Reject(err error) {
	if f.done.Swap(true) {
		panic("future: Resolve or Reject already called")
	}
	f.ch <- result[T]{err: err}
}

// IsDone returns true if the future has been resolved or rejected.
func (f *Future[T]) IsDone() bool {
	return f.done.Load()
}

// Wait blocks until the future is resolved, rejected, or the context is canceled.
func (f *Future[T]) Wait(ctx context.Context) (T, error) {
	select {
	case res := <-f.ch:
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
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return f.Wait(ctx)
}

// Reset clears the future state for reuse. Must be called before returning to pool.
func (f *Future[T]) Reset() {
	f.done.Store(false)
	// Drain the channel if there's a value
	select {
	case <-f.ch:
	default:
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
