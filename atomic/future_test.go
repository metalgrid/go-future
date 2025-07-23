package atomic

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAtomicFuture(t *testing.T) {
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

	t.Run("FastPath", func(t *testing.T) {
		fut := New[int]()
		fut.Resolve(42)

		// Multiple reads should all use fast path
		for i := 0; i < 100; i++ {
			val, err := fut.Wait(context.Background())
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			if val != 42 {
				t.Fatalf("Expected 42, got %v", val)
			}
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
}
