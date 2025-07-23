package future

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestMutexFuture(t *testing.T) {
	t.Run("Resolve", func(t *testing.T) {
		fut := New[string]()

		go func() {
			fut.Resolve("test value")
		}()

		val, err := fut.Wait(context.Background())
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if val != "test value" {
			t.Fatalf("Expected 'test value', got %v", val)
		}

		if !fut.IsDone() {
			t.Fatal("Future should be done")
		}
	})

	t.Run("Reject", func(t *testing.T) {
		fut := New[string]()
		testErr := errors.New("test error")

		go func() {
			fut.Reject(testErr)
		}()

		_, err := fut.Wait(context.Background())
		if err != testErr {
			t.Fatalf("Expected test error, got %v", err)
		}

		if !fut.IsDone() {
			t.Fatal("Future should be done")
		}
	})

	t.Run("Timeout", func(t *testing.T) {
		fut := New[string]()

		go func() {
			time.Sleep(10 * time.Millisecond)
			fut.Resolve("too late")
		}()

		_, err := fut.WaitTimeout(5 * time.Millisecond)
		if err != ErrFutureCancelled {
			t.Fatalf("Expected timeout error, got %v", err)
		}
	})

	t.Run("Pool", func(t *testing.T) {
		fut := NewFromPool[int]()

		go func() {
			fut.Resolve(42)
		}()

		val, err := fut.Wait(context.Background())
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if val != 42 {
			t.Fatalf("Expected 42, got %v", val)
		}

		Put(fut)
	})

	t.Run("AlreadyResolved", func(t *testing.T) {
		fut := New[int]()
		fut.Resolve(42)

		// Multiple reads should all return the same result
		for i := 0; i < 10; i++ {
			val, err := fut.Wait(context.Background())
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			if val != 42 {
				t.Fatalf("Expected 42, got %v", val)
			}
		}
	})

	t.Run("AlreadyRejected", func(t *testing.T) {
		fut := New[int]()
		testErr := errors.New("test error")
		fut.Reject(testErr)

		// Multiple reads should all return the same error
		for i := 0; i < 10; i++ {
			_, err := fut.Wait(context.Background())
			if err != testErr {
				t.Fatalf("Expected test error, got %v", err)
			}
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		fut := New[string]()
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(5 * time.Millisecond)
			cancel()
		}()

		_, err := fut.Wait(ctx)
		if err != ErrFutureCancelled {
			t.Fatalf("Expected cancellation error, got %v", err)
		}
	})

	t.Run("ConcurrentWaiters", func(t *testing.T) {
		fut := New[int]()
		const numWaiters = 100
		results := make([]int, numWaiters)
		errors := make([]error, numWaiters)

		var wg sync.WaitGroup
		for i := 0; i < numWaiters; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				val, err := fut.Wait(context.Background())
				results[idx] = val
				errors[idx] = err
			}(i)
		}

		// Resolve after starting all waiters
		time.Sleep(5 * time.Millisecond)
		fut.Resolve(42)

		wg.Wait()

		// All waiters should get the same result
		for i := 0; i < numWaiters; i++ {
			if errors[i] != nil {
				t.Fatalf("Waiter %d got error: %v", i, errors[i])
			}
			if results[i] != 42 {
				t.Fatalf("Waiter %d got %d, expected 42", i, results[i])
			}
		}
	})

	t.Run("DoubleResolvePanic", func(t *testing.T) {
		fut := New[int]()
		fut.Resolve(42)

		defer func() {
			if r := recover(); r == nil {
				t.Fatal("Expected panic on double resolve")
			}
		}()

		fut.Resolve(43)
	})

	t.Run("DoubleRejectPanic", func(t *testing.T) {
		fut := New[int]()
		fut.Reject(errors.New("first error"))

		defer func() {
			if r := recover(); r == nil {
				t.Fatal("Expected panic on double reject")
			}
		}()

		fut.Reject(errors.New("second error"))
	})

	t.Run("ResolveAfterRejectPanic", func(t *testing.T) {
		fut := New[int]()
		fut.Reject(errors.New("error"))

		defer func() {
			if r := recover(); r == nil {
				t.Fatal("Expected panic on resolve after reject")
			}
		}()

		fut.Resolve(42)
	})

	t.Run("RejectAfterResolvePanic", func(t *testing.T) {
		fut := New[int]()
		fut.Resolve(42)

		defer func() {
			if r := recover(); r == nil {
				t.Fatal("Expected panic on reject after resolve")
			}
		}()

		fut.Reject(errors.New("error"))
	})

	t.Run("IsDoneBeforeCompletion", func(t *testing.T) {
		fut := New[int]()

		if fut.IsDone() {
			t.Fatal("Future should not be done initially")
		}

		fut.Resolve(42)

		if !fut.IsDone() {
			t.Fatal("Future should be done after resolve")
		}
	})

	t.Run("PoolReuse", func(t *testing.T) {
		// Test that pooled futures can be reused properly
		fut1 := NewFromPool[string]()
		fut1.Resolve("first")

		val, err := fut1.Wait(context.Background())
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if val != "first" {
			t.Fatalf("Expected 'first', got %v", val)
		}

		Put(fut1)

		// Get another future from pool (might be the same one)
		fut2 := NewFromPool[string]()
		if fut2.IsDone() {
			t.Fatal("Pooled future should be reset and not done")
		}

		fut2.Resolve("second")
		val2, err2 := fut2.Wait(context.Background())
		if err2 != nil {
			t.Fatalf("Expected no error, got %v", err2)
		}
		if val2 != "second" {
			t.Fatalf("Expected 'second', got %v", val2)
		}

		Put(fut2)
	})

	t.Run("NilPutHandling", func(t *testing.T) {
		// Should not panic when putting nil
		Put[int](nil)
	})

	t.Run("ZeroValueHandling", func(t *testing.T) {
		fut := New[string]()
		fut.Resolve("") // Zero value for string

		val, err := fut.Wait(context.Background())
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if val != "" {
			t.Fatalf("Expected empty string, got %v", val)
		}
	})

	t.Run("StructTypeHandling", func(t *testing.T) {
		type TestStruct struct {
			Name string
			Age  int
		}

		fut := New[TestStruct]()
		expected := TestStruct{Name: "test", Age: 25}

		go func() {
			fut.Resolve(expected)
		}()

		val, err := fut.Wait(context.Background())
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if val != expected {
			t.Fatalf("Expected %+v, got %+v", expected, val)
		}
	})

	t.Run("PointerTypeHandling", func(t *testing.T) {
		fut := New[*string]()
		str := "test string"

		go func() {
			fut.Resolve(&str)
		}()

		val, err := fut.Wait(context.Background())
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if val == nil {
			t.Fatal("Expected non-nil pointer")
		}
		if *val != str {
			t.Fatalf("Expected %s, got %s", str, *val)
		}
	})
}
