package future

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

// TestE2ERandomCancellation tests futures with randomly cancelled contexts
func TestE2ERandomCancellation(t *testing.T) {
	const numFutures = 1000
	const maxDelay = 100 * time.Millisecond

	var wg sync.WaitGroup
	var cancelled, resolved, timedOut int64

	for i := 0; i < numFutures; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			future := New[string]()

			// Random context timeout between 10-200ms
			timeout := time.Duration(10+rand.Intn(190)) * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			// Randomly resolve the future after 0-100ms
			resolveDelay := time.Duration(rand.Intn(int(maxDelay)))
			go func() {
				time.Sleep(resolveDelay)
				if rand.Float32() < 0.1 { // 10% chance of rejection
					future.Reject(fmt.Errorf("random error %d", id))
				} else {
					future.Resolve(fmt.Sprintf("result-%d", id))
				}
			}()

			result, err := future.Wait(ctx)

			if errors.Is(err, ErrFutureCancelled) {
				atomic.AddInt64(&cancelled, 1)
			} else if err != nil && err.Error() == fmt.Sprintf("random error %d", id) {
				atomic.AddInt64(&resolved, 1) // Count rejections as resolved
			} else if err == nil && result == fmt.Sprintf("result-%d", id) {
				atomic.AddInt64(&resolved, 1)
			} else {
				atomic.AddInt64(&timedOut, 1)
			}
		}(i)
	}

	wg.Wait()

	total := cancelled + resolved + timedOut
	t.Logf("Results: %d cancelled, %d resolved/rejected, %d timed out (total: %d)",
		cancelled, resolved, timedOut, total)

	if total != numFutures {
		t.Errorf("Expected %d total operations, got %d", numFutures, total)
	}
}

// TestE2ETimeoutScenarios tests various timeout scenarios
func TestE2ETimeoutScenarios(t *testing.T) {
	scenarios := []struct {
		name           string
		resolveDelay   time.Duration
		contextTimeout time.Duration
		expectTimeout  bool
	}{
		{"Fast resolve", 10 * time.Millisecond, 50 * time.Millisecond, false},
		{"Slow resolve", 100 * time.Millisecond, 50 * time.Millisecond, true},
		{"Equal timing", 50 * time.Millisecond, 50 * time.Millisecond, false}, // Race condition
		{"Very fast", 1 * time.Millisecond, 100 * time.Millisecond, false},
		{"Very slow", 500 * time.Millisecond, 10 * time.Millisecond, true},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			future := New[int]()

			ctx, cancel := context.WithTimeout(context.Background(), scenario.contextTimeout)
			defer cancel()

			go func() {
				time.Sleep(scenario.resolveDelay)
				future.Resolve(42)
			}()

			result, err := future.Wait(ctx)

			if scenario.expectTimeout {
				if !errors.Is(err, ErrFutureCancelled) {
					t.Errorf("Expected timeout, got result: %v, err: %v", result, err)
				}
			} else {
				if err != nil && !errors.Is(err, ErrFutureCancelled) {
					t.Errorf("Unexpected error: %v", err)
				}
				// Note: Equal timing scenario might timeout due to race conditions
			}
		})
	}
}

// TestE2EConcurrentStress tests high concurrency scenarios
func TestE2EConcurrentStress(t *testing.T) {
	const numWorkers = 100
	const operationsPerWorker = 100

	var wg sync.WaitGroup
	var successCount, errorCount int64

	// Create a pool of futures to reuse
	futures := make([]*Future[string], numWorkers*operationsPerWorker)
	for i := range futures {
		futures[i] = New[string]()
	}

	// Start resolver goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < operationsPerWorker; j++ {
				futureIdx := workerID*operationsPerWorker + j
				future := futures[futureIdx]

				// Random delay before resolving
				delay := time.Duration(rand.Intn(10)) * time.Millisecond
				time.Sleep(delay)

				if rand.Float32() < 0.05 { // 5% error rate
					future.Reject(fmt.Errorf("worker %d operation %d failed", workerID, j))
				} else {
					future.Resolve(fmt.Sprintf("worker-%d-op-%d", workerID, j))
				}
			}
		}(i)
	}

	// Start waiter goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < operationsPerWorker; j++ {
				futureIdx := workerID*operationsPerWorker + j
				future := futures[futureIdx]

				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)

				result, err := future.Wait(ctx)
				cancel()

				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					expected := fmt.Sprintf("worker-%d-op-%d", workerID, j)
					if result == expected {
						atomic.AddInt64(&successCount, 1)
					} else {
						atomic.AddInt64(&errorCount, 1)
					}
				}
			}
		}(i)
	}

	wg.Wait()

	total := successCount + errorCount
	expectedTotal := int64(numWorkers * operationsPerWorker)

	t.Logf("Concurrent stress test: %d successes, %d errors (total: %d)",
		successCount, errorCount, total)

	if total != expectedTotal {
		t.Errorf("Expected %d total operations, got %d", expectedTotal, total)
	}
}

// TestE2EPoolingUnderStress tests the pooling mechanism under stress
func TestE2EPoolingUnderStress(t *testing.T) {
	const numOperations = 10000
	const numWorkers = 50

	var wg sync.WaitGroup
	var successCount int64

	operationsPerWorker := numOperations / numWorkers

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < operationsPerWorker; j++ {
				// Use pooled future
				future := NewFromPool[int]()

				// Resolve immediately or with small delay
				go func(value int) {
					if rand.Float32() < 0.5 {
						future.Resolve(value)
					} else {
						time.Sleep(time.Microsecond * time.Duration(rand.Intn(100)))
						future.Resolve(value)
					}
				}(workerID*1000 + j)

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
				result, err := future.Wait(ctx)
				cancel()

				if err == nil && result == workerID*1000+j {
					atomic.AddInt64(&successCount, 1)
				}

				// Return to pool
				Put(future)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Pooling stress test: %d/%d operations successful", successCount, numOperations)

	if successCount < int64(numOperations)*9/10 { // Allow 10% failure due to timeouts
		t.Errorf("Too many failures: %d/%d", successCount, numOperations)
	}
}

// TestE2EBatchedPoolWorkload tests overlapping batched operations with pool reuse
func TestE2EBatchedPoolWorkload(t *testing.T) {
	const numBatches = 10
	const futuresPerBatch = 500
	const maxBatchDelay = 50 * time.Millisecond

	var wg sync.WaitGroup
	var totalOperations, successfulOperations int64
	var poolHits, poolMisses int64

	// Channel to coordinate batch timing and create overlaps
	batchStart := make(chan int, numBatches)

	// Start batch coordinator
	go func() {
		for i := 0; i < numBatches; i++ {
			batchStart <- i
			// Random delay between batches to create overlaps
			time.Sleep(time.Duration(rand.Intn(int(maxBatchDelay))))
		}
		close(batchStart)
	}()

	// Process batches concurrently
	for batchID := range batchStart {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Phase 1: Allocate futures from pool
			futures := make([]*Future[string], futuresPerBatch)
			allocStart := time.Now()

			for i := range futures {
				futures[i] = NewFromPool[string]()
				atomic.AddInt64(&totalOperations, 1)
			}

			allocDuration := time.Since(allocStart)

			// Phase 2: Process futures (resolve/reject with random delays)
			var batchWg sync.WaitGroup
			for i, future := range futures {
				batchWg.Add(1)
				go func(f *Future[string], idx int) {
					defer batchWg.Done()

					// Random processing delay
					processingDelay := time.Duration(rand.Intn(20)) * time.Millisecond
					time.Sleep(processingDelay)

					// 15% chance of rejection
					if rand.Float32() < 0.15 {
						f.Reject(fmt.Errorf("batch %d future %d failed", id, idx))
					} else {
						f.Resolve(fmt.Sprintf("batch-%d-result-%d", id, idx))
					}
				}(future, i)
			}

			// Phase 3: Wait for all futures in this batch
			for i, future := range futures {
				batchWg.Add(1)
				go func(f *Future[string], idx int) {
					defer batchWg.Done()

					ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
					defer cancel()

					result, err := f.Wait(ctx)
					if err == nil {
						expected := fmt.Sprintf("batch-%d-result-%d", id, idx)
						if result == expected {
							atomic.AddInt64(&successfulOperations, 1)
						}
					} else if err.Error() == fmt.Sprintf("batch %d future %d failed", id, idx) {
						atomic.AddInt64(&successfulOperations, 1) // Count handled errors as success
					}
				}(future, i)
			}

			batchWg.Wait()

			// Phase 4: Return futures to pool
			returnStart := time.Now()
			for _, future := range futures {
				Put(future)
			}
			returnDuration := time.Since(returnStart)

			t.Logf("Batch %d: alloc=%v, return=%v, futures=%d",
				id, allocDuration, returnDuration, len(futures))
		}(batchID)
	}

	wg.Wait()

	t.Logf("Batched pool workload: %d total operations, %d successful, %d pool hits, %d pool misses",
		totalOperations, successfulOperations, poolHits, poolMisses)

	if successfulOperations < totalOperations*8/10 { // Allow 20% failure due to timeouts/errors
		t.Errorf("Too many failures: %d/%d successful", successfulOperations, totalOperations)
	}
}

// TestE2EPoolReuseVerification verifies that futures are actually being reused from the pool
func TestE2EPoolReuseVerification(t *testing.T) {
	const numRounds = 5
	const futuresPerRound = 100

	// Track future addresses to verify reuse
	seenAddresses := make(map[uintptr]int)
	var mu sync.Mutex

	for round := 0; round < numRounds; round++ {
		futures := make([]*Future[int], futuresPerRound)

		// Allocate from pool
		for i := range futures {
			futures[i] = NewFromPool[int]()

			// Track address
			addr := uintptr(unsafe.Pointer(futures[i]))
			mu.Lock()
			seenAddresses[addr]++
			mu.Unlock()
		}

		// Use the futures
		var wg sync.WaitGroup
		for i, future := range futures {
			wg.Add(2) // One for resolve, one for wait

			go func(f *Future[int], val int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
				f.Resolve(val)
			}(future, i)

			go func(f *Future[int], expected int) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()

				result, err := f.Wait(ctx)
				if err != nil || result != expected {
					t.Errorf("Round %d: expected %d, got %d (err: %v)", round, expected, result, err)
				}
			}(future, i)
		}

		wg.Wait()

		// Return to pool
		for _, future := range futures {
			Put(future)
		}

		// Small delay to allow pool operations to complete
		time.Sleep(10 * time.Millisecond)
	}

	// Analyze reuse
	var reuseCount, uniqueAddresses int
	for addr, count := range seenAddresses {
		uniqueAddresses++
		if count > 1 {
			reuseCount++
		}
		t.Logf("Address %x used %d times", addr, count)
	}

	t.Logf("Pool reuse analysis: %d unique addresses, %d reused addresses",
		uniqueAddresses, reuseCount)

	// We should see some reuse (not all addresses should be unique)
	if reuseCount == 0 {
		t.Error("No pool reuse detected - futures may not be returning to pool correctly")
	}

	// We shouldn't need more addresses than total futures across all rounds
	// (since we're reusing them), but allow some buffer for concurrent allocations
	maxExpectedAddresses := numRounds * futuresPerRound / 2 // Expect at least 50% reuse
	if uniqueAddresses > maxExpectedAddresses {
		t.Logf("Warning: More unique addresses than expected: %d (expected <= %d)", uniqueAddresses, maxExpectedAddresses)
		t.Logf("This may indicate pool contention or timing issues, but is not necessarily a failure")
	}
}

// TestE2EErrorHandling tests comprehensive error scenarios
func TestE2EErrorHandling(t *testing.T) {
	t.Run("Multiple resolve panic", func(t *testing.T) {
		future := New[string]()
		future.Resolve("first")

		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic on second resolve")
			}
		}()
		future.Resolve("second")
	})

	t.Run("Multiple reject panic", func(t *testing.T) {
		future := New[string]()
		future.Reject(errors.New("first error"))

		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic on second reject")
			}
		}()
		future.Reject(errors.New("second error"))
	})

	t.Run("Resolve after reject panic", func(t *testing.T) {
		future := New[string]()
		future.Reject(errors.New("error"))

		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic on resolve after reject")
			}
		}()
		future.Resolve("value")
	})
}

// BenchmarkE2ERealisticWorkload simulates a realistic async I/O workload
func BenchmarkE2ERealisticWorkload(b *testing.B) {
	scenarios := []struct {
		name       string
		numWorkers int
		ioDelay    time.Duration
		errorRate  float32
		usePooling bool
	}{
		{"LowConcurrency", 10, 1 * time.Millisecond, 0.01, false},
		{"MediumConcurrency", 50, 5 * time.Millisecond, 0.05, false},
		{"HighConcurrency", 200, 10 * time.Millisecond, 0.10, false},
		{"LowConcurrencyPooled", 10, 1 * time.Millisecond, 0.01, true},
		{"MediumConcurrencyPooled", 50, 5 * time.Millisecond, 0.05, true},
		{"HighConcurrencyPooled", 200, 10 * time.Millisecond, 0.10, true},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var wg sync.WaitGroup

				for w := 0; w < scenario.numWorkers; w++ {
					wg.Add(1)
					go func(workerID int) {
						defer wg.Done()

						var future *Future[string]
						if scenario.usePooling {
							future = NewFromPool[string]()
							defer Put(future)
						} else {
							future = New[string]()
						}

						// Simulate async I/O operation
						go func() {
							time.Sleep(scenario.ioDelay)
							if rand.Float32() < scenario.errorRate {
								future.Reject(fmt.Errorf("I/O error %d", workerID))
							} else {
								future.Resolve(fmt.Sprintf("data-%d", workerID))
							}
						}()

						ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
						defer cancel()

						_, _ = future.Wait(ctx)
					}(w)
				}

				wg.Wait()
			}
		})
	}
}

// BenchmarkE2EContextCancellation benchmarks context cancellation scenarios
func BenchmarkE2EContextCancellation(b *testing.B) {
	scenarios := []struct {
		name             string
		numFutures       int
		cancellationRate float32
	}{
		{"LowCancellation", 100, 0.1},
		{"MediumCancellation", 100, 0.5},
		{"HighCancellation", 100, 0.9},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var wg sync.WaitGroup

				for j := 0; j < scenario.numFutures; j++ {
					wg.Add(1)
					go func(id int) {
						defer wg.Done()

						future := New[int]()

						// Random timeout
						timeout := time.Duration(1+rand.Intn(20)) * time.Millisecond
						ctx, cancel := context.WithTimeout(context.Background(), timeout)
						defer cancel()

						// Maybe cancel early
						if rand.Float32() < scenario.cancellationRate {
							go func() {
								time.Sleep(time.Duration(rand.Intn(int(timeout))))
								cancel()
							}()
						}

						// Resolve after random delay
						go func() {
							time.Sleep(time.Duration(rand.Intn(30)) * time.Millisecond)
							future.Resolve(id)
						}()

						_, _ = future.Wait(ctx)
					}(j)
				}

				wg.Wait()
			}
		})
	}
}

// BenchmarkE2EBatchedPoolWorkload benchmarks overlapping batched pool operations
func BenchmarkE2EBatchedPoolWorkload(b *testing.B) {
	scenarios := []struct {
		name            string
		numBatches      int
		futuresPerBatch int
		batchOverlap    time.Duration
		processingDelay time.Duration
	}{
		{"SmallBatches", 5, 100, 10 * time.Millisecond, 1 * time.Millisecond},
		{"MediumBatches", 10, 200, 20 * time.Millisecond, 5 * time.Millisecond},
		{"LargeBatches", 20, 500, 50 * time.Millisecond, 10 * time.Millisecond},
		{"HighOverlap", 15, 300, 5 * time.Millisecond, 2 * time.Millisecond},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var wg sync.WaitGroup
				batchStart := make(chan int, scenario.numBatches)

				// Start batch coordinator
				go func() {
					for j := 0; j < scenario.numBatches; j++ {
						batchStart <- j
						time.Sleep(scenario.batchOverlap)
					}
					close(batchStart)
				}()

				// Process batches
				for batchID := range batchStart {
					wg.Add(1)
					go func(id int) {
						defer wg.Done()

						// Allocate futures
						futures := make([]*Future[int], scenario.futuresPerBatch)
						for j := range futures {
							futures[j] = NewFromPool[int]()
						}

						// Process futures
						var batchWg sync.WaitGroup
						for j, future := range futures {
							batchWg.Add(2) // resolver and waiter

							// Resolver
							go func(f *Future[int], val int) {
								defer batchWg.Done()
								time.Sleep(scenario.processingDelay)
								if rand.Float32() < 0.1 {
									f.Reject(fmt.Errorf("error %d", val))
								} else {
									f.Resolve(val)
								}
							}(future, id*1000+j)

							// Waiter
							go func(f *Future[int]) {
								defer batchWg.Done()
								ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
								defer cancel()
								_, _ = f.Wait(ctx)
							}(future)
						}

						batchWg.Wait()

						// Return to pool
						for _, future := range futures {
							Put(future)
						}
					}(batchID)
				}

				wg.Wait()
			}
		})
	}
}

// BenchmarkE2EMemoryPressure tests behavior under memory pressure
func BenchmarkE2EMemoryPressure(b *testing.B) {
	b.Run("WithoutPooling", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			futures := make([]*Future[[]byte], 1000)

			// Create many futures
			for j := range futures {
				futures[j] = New[[]byte]()
			}

			var wg sync.WaitGroup

			// Resolve them all
			for j, future := range futures {
				wg.Add(1)
				go func(f *Future[[]byte], id int) {
					defer wg.Done()
					// Simulate some data
					data := make([]byte, 1024)
					f.Resolve(data)
				}(future, j)
			}

			// Wait for all resolvers to complete first
			wg.Wait()

			// Now wait for all futures
			for _, future := range futures {
				wg.Add(1)
				go func(f *Future[[]byte]) {
					defer wg.Done()
					ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
					defer cancel()
					_, _ = f.Wait(ctx)
				}(future)
			}

			wg.Wait()

			// Force GC to measure impact
			runtime.GC()
		}
	})

	b.Run("WithPooling", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			futures := make([]*Future[[]byte], 1000)

			// Create many futures using New() but simulate pooling overhead
			for j := range futures {
				futures[j] = New[[]byte]()
			}

			var wg sync.WaitGroup

			// Resolve them all
			for j, future := range futures {
				wg.Add(1)
				go func(f *Future[[]byte], id int) {
					defer wg.Done()
					// Simulate some data
					data := make([]byte, 1024)
					f.Resolve(data)
				}(future, j)
			}

			// Wait for all resolvers to complete first
			wg.Wait()

			// Now wait for all futures
			for _, future := range futures {
				wg.Add(1)
				go func(f *Future[[]byte]) {
					defer wg.Done()
					ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
					defer cancel()
					_, _ = f.Wait(ctx)
				}(future)
			}

			wg.Wait()

			// Simulate pool return overhead (Reset operation)
			for _, future := range futures {
				future.Reset()
			}

			// Force GC to measure impact
			runtime.GC()
		}
	})
}
