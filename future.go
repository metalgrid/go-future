package future

import (
	"context"
	"errors"
	"sync"
	"time"
	"unsafe"
)

var ErrFutureCancelled = errors.New("future: context cancelled before resolution")

var futurePool = sync.Pool{
	New: func() interface{} {
		f := &Future[any]{}
		f.cond = sync.NewCond(&f.mu)
		return f
	},
}

type Future[T any] struct {
	mu    sync.Mutex
	cond  *sync.Cond
	value T
	err   error
	done  bool
}

// New creates a new unresolved Future.
func New[T any]() *Future[T] {
	f := &Future[T]{}
	f.cond = sync.NewCond(&f.mu)
	return f
}

// NewFromPool creates a new unresolved Future from the pool for better performance.
func NewFromPool[T any]() *Future[T] {
	return Get[T]()
}

// Resolve completes the Future with a value.
// It must not be called after Resolve or Reject.
func (f *Future[T]) Resolve(v T) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.done {
		panic("future: Resolve or Reject already called")
	}
	f.value = v
	f.done = true
	f.cond.Broadcast()
}

// Reject completes the Future with an error.
// It must not be called after Resolve or Reject.
func (f *Future[T]) Reject(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.done {
		panic("future: Resolve or Reject already called")
	}
	f.err = err
	f.done = true
	f.cond.Broadcast()
}

// IsDone returns true if the future has been resolved or rejected.
func (f *Future[T]) IsDone() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.done
}

// Wait blocks until the future is resolved, rejected, or the context is canceled.
func (f *Future[T]) Wait(ctx context.Context) (T, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.done {
		return f.returnResult()
	}

	cancel := context.AfterFunc(ctx, func() {
		f.cond.Broadcast()
	})
	defer cancel()

	for !f.done {
		f.cond.Wait()
		if ctx.Err() != nil {
			var zero T
			return zero, ErrFutureCancelled
		}
	}

	return f.returnResult()
}

func (f *Future[T]) WaitTimeout(timeout time.Duration) (T, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return f.Wait(ctx)
}

// returnResult returns either the resolved value or the rejection error.
func (f *Future[T]) returnResult() (T, error) {
	if f.err != nil {
		var zero T
		return zero, f.err
	}
	return f.value, nil
}

// Reset clears the future state for reuse. Must be called before returning to pool.
func (f *Future[T]) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	var zero T
	f.value = zero
	f.err = nil
	f.done = false
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
