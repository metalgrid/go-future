package future

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"
)

func BenchmarkMillionFutures(b *testing.B) {
	const numFutures = 1_000_000

	b.Run("CreateAndResolve", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			futures := make([]*Future[int], numFutures)

			// Create a million futures
			for j := 0; j < numFutures; j++ {
				futures[j] = New[int]()
			}

			// Resolve all futures concurrently
			var wg sync.WaitGroup
			for j := 0; j < numFutures; j++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					futures[idx].Resolve(idx)
				}(j)
			}
			wg.Wait()
		}
	})

	b.Run("CreateResolveAndWait", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			futures := make([]*Future[int], numFutures)
			ctx := context.Background()

			// Create a million futures
			for j := 0; j < numFutures; j++ {
				futures[j] = New[int]()
			}

			// Resolve all futures concurrently
			var resolveWg sync.WaitGroup
			for j := 0; j < numFutures; j++ {
				resolveWg.Add(1)
				go func(idx int) {
					defer resolveWg.Done()
					futures[idx].Resolve(idx)
				}(j)
			}

			// Wait on all futures concurrently
			var waitWg sync.WaitGroup
			for j := 0; j < numFutures; j++ {
				waitWg.Add(1)
				go func(idx int) {
					defer waitWg.Done()
					_, _ = futures[idx].Wait(ctx)
				}(j)
			}

			resolveWg.Wait()
			waitWg.Wait()
		}
	})

	b.Run("MemoryUsage", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			futures := make([]*Future[string], numFutures)

			// Create futures
			for j := 0; j < numFutures; j++ {
				futures[j] = New[string]()
			}

			// Resolve a subset to test mixed states
			for j := 0; j < numFutures/2; j++ {
				futures[j].Resolve("resolved")
			}

			// Force GC to measure actual memory usage
			runtime.GC()
		}
	})
}

func BenchmarkFutureOperations(b *testing.B) {
	b.Run("SingleFutureResolveWait", func(b *testing.B) {
		ctx := context.Background()

		for i := 0; i < b.N; i++ {
			fut := New[int]()

			go func() {
				fut.Resolve(42)
			}()

			_, _ = fut.Wait(ctx)
		}
	})

	b.Run("SingleFutureTimeout", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fut := New[int]()

			go func() {
				time.Sleep(10 * time.Millisecond)
				fut.Resolve(42)
			}()

			_, _ = fut.WaitTimeout(5 * time.Millisecond)
		}
	})

	b.Run("IsDoneCheck", func(b *testing.B) {
		fut := New[int]()
		fut.Resolve(42)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = fut.IsDone()
		}
	})
}
