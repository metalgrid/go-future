package future

import (
	"context"
	"github.com/metalgrid/go-future/atomic"
	"github.com/metalgrid/go-future/ch"
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

func BenchmarkPooledVsNonPooled(b *testing.B) {
	const numFutures = 100_000

	b.Run("NonPooled", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			futures := make([]*Future[int], numFutures)

			for j := 0; j < numFutures; j++ {
				futures[j] = New[int]()
			}

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

	b.Run("Pooled", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			futures := make([]*Future[int], numFutures)

			for j := 0; j < numFutures; j++ {
				futures[j] = NewFromPool[int]()
			}

			var wg sync.WaitGroup
			for j := 0; j < numFutures; j++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					futures[idx].Resolve(idx)
				}(j)
			}
			wg.Wait()

			// Return to pool
			for j := 0; j < numFutures; j++ {
				Put(futures[j])
			}
		}
	})

	b.Run("PooledWithReuse", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Create and use futures multiple times to show pool benefits
			for round := 0; round < 5; round++ {
				futures := make([]*Future[int], numFutures/5)

				for j := 0; j < numFutures/5; j++ {
					futures[j] = NewFromPool[int]()
				}

				var wg sync.WaitGroup
				for j := 0; j < numFutures/5; j++ {
					wg.Add(1)
					go func(idx int) {
						defer wg.Done()
						futures[idx].Resolve(idx)
					}(j)
				}
				wg.Wait()

				// Return to pool for next round
				for j := 0; j < numFutures/5; j++ {
					Put(futures[j])
				}
			}
		}
	})
}

func BenchmarkMutexVsChannel(b *testing.B) {
	const numFutures = 100_000

	b.Run("MutexBased", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			futures := make([]*Future[int], numFutures)

			for j := 0; j < numFutures; j++ {
				futures[j] = New[int]()
			}

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

	b.Run("ChannelBased", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			futures := make([]*ch.Future[int], numFutures)

			for j := 0; j < numFutures; j++ {
				futures[j] = ch.New[int]()
			}

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

	b.Run("MutexPooled", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			futures := make([]*Future[int], numFutures)

			for j := 0; j < numFutures; j++ {
				futures[j] = NewFromPool[int]()
			}

			var wg sync.WaitGroup
			for j := 0; j < numFutures; j++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					futures[idx].Resolve(idx)
				}(j)
			}
			wg.Wait()

			for j := 0; j < numFutures; j++ {
				Put(futures[j])
			}
		}
	})

	b.Run("ChannelPooled", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			futures := make([]*ch.Future[int], numFutures)

			for j := 0; j < numFutures; j++ {
				futures[j] = ch.NewFromPool[int]()
			}

			var wg sync.WaitGroup
			for j := 0; j < numFutures; j++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					futures[idx].Resolve(idx)
				}(j)
			}
			wg.Wait()

			for j := 0; j < numFutures; j++ {
				ch.Put(futures[j])
			}
		}
	})

	b.Run("MutexWithWait", func(b *testing.B) {
		b.ReportAllocs()
		ctx := context.Background()

		for i := 0; i < b.N; i++ {
			futures := make([]*Future[int], numFutures)

			for j := 0; j < numFutures; j++ {
				futures[j] = NewFromPool[int]()
			}

			var resolveWg sync.WaitGroup
			for j := 0; j < numFutures; j++ {
				resolveWg.Add(1)
				go func(idx int) {
					defer resolveWg.Done()
					futures[idx].Resolve(idx)
				}(j)
			}

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

			for j := 0; j < numFutures; j++ {
				Put(futures[j])
			}
		}
	})

	b.Run("ChannelWithWait", func(b *testing.B) {
		b.ReportAllocs()
		ctx := context.Background()

		for i := 0; i < b.N; i++ {
			futures := make([]*ch.Future[int], numFutures)

			for j := 0; j < numFutures; j++ {
				futures[j] = ch.NewFromPool[int]()
			}

			var resolveWg sync.WaitGroup
			for j := 0; j < numFutures; j++ {
				resolveWg.Add(1)
				go func(idx int) {
					defer resolveWg.Done()
					futures[idx].Resolve(idx)
				}(j)
			}

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

			for j := 0; j < numFutures; j++ {
				ch.Put(futures[j])
			}
		}
	})
}

func BenchmarkGCPressure(b *testing.B) {
	const numFutures = 500_000

	b.Run("MutexGCPressure", func(b *testing.B) {
		b.ReportAllocs()

		var startGC, endGC runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&startGC)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			futures := make([]*Future[int], numFutures)

			// Create futures
			for j := 0; j < numFutures; j++ {
				futures[j] = New[int]()
			}

			// Resolve futures
			var wg sync.WaitGroup
			for j := 0; j < numFutures; j++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					futures[idx].Resolve(idx)
				}(j)
			}
			wg.Wait()

			// Force GC and measure
			runtime.GC()
		}
		b.StopTimer()

		runtime.ReadMemStats(&endGC)
		b.ReportMetric(float64(endGC.NumGC-startGC.NumGC), "gc-cycles")
		b.ReportMetric(float64(endGC.PauseTotalNs-startGC.PauseTotalNs)/1e6, "gc-pause-ms")
		b.ReportMetric(float64(endGC.TotalAlloc-startGC.TotalAlloc)/1e6, "total-alloc-mb")
	})

	b.Run("ChannelGCPressure", func(b *testing.B) {
		b.ReportAllocs()

		var startGC, endGC runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&startGC)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			futures := make([]*ch.Future[int], numFutures)

			// Create futures
			for j := 0; j < numFutures; j++ {
				futures[j] = ch.New[int]()
			}

			// Resolve futures
			var wg sync.WaitGroup
			for j := 0; j < numFutures; j++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					futures[idx].Resolve(idx)
				}(j)
			}
			wg.Wait()

			// Force GC and measure
			runtime.GC()
		}
		b.StopTimer()

		runtime.ReadMemStats(&endGC)
		b.ReportMetric(float64(endGC.NumGC-startGC.NumGC), "gc-cycles")
		b.ReportMetric(float64(endGC.PauseTotalNs-startGC.PauseTotalNs)/1e6, "gc-pause-ms")
		b.ReportMetric(float64(endGC.TotalAlloc-startGC.TotalAlloc)/1e6, "total-alloc-mb")
	})

	b.Run("MutexPooledGCPressure", func(b *testing.B) {
		b.ReportAllocs()

		var startGC, endGC runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&startGC)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Multiple rounds to show pool benefits
			for round := 0; round < 3; round++ {
				futures := make([]*Future[int], numFutures/3)

				// Create futures from pool
				for j := 0; j < numFutures/3; j++ {
					futures[j] = NewFromPool[int]()
				}

				// Resolve futures
				var wg sync.WaitGroup
				for j := 0; j < numFutures/3; j++ {
					wg.Add(1)
					go func(idx int) {
						defer wg.Done()
						futures[idx].Resolve(idx)
					}(j)
				}
				wg.Wait()

				// Return to pool
				for j := 0; j < numFutures/3; j++ {
					Put(futures[j])
				}
			}

			// Force GC and measure
			runtime.GC()
		}
		b.StopTimer()

		runtime.ReadMemStats(&endGC)
		b.ReportMetric(float64(endGC.NumGC-startGC.NumGC), "gc-cycles")
		b.ReportMetric(float64(endGC.PauseTotalNs-startGC.PauseTotalNs)/1e6, "gc-pause-ms")
		b.ReportMetric(float64(endGC.TotalAlloc-startGC.TotalAlloc)/1e6, "total-alloc-mb")
	})

	b.Run("ChannelPooledGCPressure", func(b *testing.B) {
		b.ReportAllocs()

		var startGC, endGC runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&startGC)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Multiple rounds to show pool benefits
			for round := 0; round < 3; round++ {
				futures := make([]*ch.Future[int], numFutures/3)

				// Create futures from pool
				for j := 0; j < numFutures/3; j++ {
					futures[j] = ch.NewFromPool[int]()
				}

				// Resolve futures
				var wg sync.WaitGroup
				for j := 0; j < numFutures/3; j++ {
					wg.Add(1)
					go func(idx int) {
						defer wg.Done()
						futures[idx].Resolve(idx)
					}(j)
				}
				wg.Wait()

				// Return to pool
				for j := 0; j < numFutures/3; j++ {
					ch.Put(futures[j])
				}
			}

			// Force GC and measure
			runtime.GC()
		}
		b.StopTimer()

		runtime.ReadMemStats(&endGC)
		b.ReportMetric(float64(endGC.NumGC-startGC.NumGC), "gc-cycles")
		b.ReportMetric(float64(endGC.PauseTotalNs-startGC.PauseTotalNs)/1e6, "gc-pause-ms")
		b.ReportMetric(float64(endGC.TotalAlloc-startGC.TotalAlloc)/1e6, "total-alloc-mb")
	})
}

func BenchmarkGCStress(b *testing.B) {
	b.Run("MutexMemoryStress", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Simulate high-frequency future creation/destruction
			const batchSize = 10_000
			const batches = 50

			for batch := 0; batch < batches; batch++ {
				futures := make([]*Future[string], batchSize)

				// Create and immediately resolve
				for j := 0; j < batchSize; j++ {
					futures[j] = New[string]()
					go func(idx int) {
						futures[idx].Resolve("data")
					}(j)
				}

				// Wait for all to complete
				ctx := context.Background()
				for j := 0; j < batchSize; j++ {
					_, _ = futures[j].Wait(ctx)
				}

				// Let futures become eligible for GC
				futures = nil

				// Trigger GC every few batches
				if batch%10 == 0 {
					runtime.GC()
				}
			}
		}
	})

	b.Run("ChannelMemoryStress", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Simulate high-frequency future creation/destruction
			const batchSize = 10_000
			const batches = 50

			for batch := 0; batch < batches; batch++ {
				futures := make([]*ch.Future[string], batchSize)

				// Create and immediately resolve
				for j := 0; j < batchSize; j++ {
					futures[j] = ch.New[string]()
					go func(idx int) {
						futures[idx].Resolve("data")
					}(j)
				}

				// Wait for all to complete
				ctx := context.Background()
				for j := 0; j < batchSize; j++ {
					_, _ = futures[j].Wait(ctx)
				}

				// Let futures become eligible for GC
				futures = nil

				// Trigger GC every few batches
				if batch%10 == 0 {
					runtime.GC()
				}
			}
		}
	})

	b.Run("PooledMemoryStress", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Simulate high-frequency future creation/destruction with pooling
			const batchSize = 10_000
			const batches = 50

			for batch := 0; batch < batches; batch++ {
				futures := make([]*Future[string], batchSize)

				// Create from pool and immediately resolve
				for j := 0; j < batchSize; j++ {
					futures[j] = NewFromPool[string]()
					go func(idx int) {
						futures[idx].Resolve("data")
					}(j)
				}

				// Wait for all to complete
				ctx := context.Background()
				for j := 0; j < batchSize; j++ {
					_, _ = futures[j].Wait(ctx)
				}

				// Return to pool
				for j := 0; j < batchSize; j++ {
					Put(futures[j])
				}

				// Trigger GC every few batches
				if batch%10 == 0 {
					runtime.GC()
				}
			}
		}
	})
}

func BenchmarkThreeWayComparison(b *testing.B) {
	const numFutures = 100_000

	b.Run("MutexBased", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			futures := make([]*Future[int], numFutures)

			for j := 0; j < numFutures; j++ {
				futures[j] = New[int]()
			}

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

	b.Run("ChannelBased", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			futures := make([]*ch.Future[int], numFutures)

			for j := 0; j < numFutures; j++ {
				futures[j] = ch.New[int]()
			}

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

	b.Run("AtomicHybrid", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			futures := make([]*atomic.Future[int], numFutures)

			for j := 0; j < numFutures; j++ {
				futures[j] = atomic.New[int]()
			}

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

	b.Run("MutexPooled", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			futures := make([]*Future[int], numFutures)

			for j := 0; j < numFutures; j++ {
				futures[j] = NewFromPool[int]()
			}

			var wg sync.WaitGroup
			for j := 0; j < numFutures; j++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					futures[idx].Resolve(idx)
				}(j)
			}
			wg.Wait()

			for j := 0; j < numFutures; j++ {
				Put(futures[j])
			}
		}
	})

	b.Run("ChannelPooled", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			futures := make([]*ch.Future[int], numFutures)

			for j := 0; j < numFutures; j++ {
				futures[j] = ch.NewFromPool[int]()
			}

			var wg sync.WaitGroup
			for j := 0; j < numFutures; j++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					futures[idx].Resolve(idx)
				}(j)
			}
			wg.Wait()

			for j := 0; j < numFutures; j++ {
				ch.Put(futures[j])
			}
		}
	})

	b.Run("AtomicPooled", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			futures := make([]*atomic.Future[int], numFutures)

			for j := 0; j < numFutures; j++ {
				futures[j] = atomic.NewFromPool[int]()
			}

			var wg sync.WaitGroup
			for j := 0; j < numFutures; j++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					futures[idx].Resolve(idx)
				}(j)
			}
			wg.Wait()

			for j := 0; j < numFutures; j++ {
				atomic.Put(futures[j])
			}
		}
	})
}

func BenchmarkFastPathPerformance(b *testing.B) {
	b.Run("MutexIsDone", func(b *testing.B) {
		fut := New[int]()
		fut.Resolve(42)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = fut.IsDone()
		}
	})

	b.Run("ChannelIsDone", func(b *testing.B) {
		fut := ch.New[int]()
		fut.Resolve(42)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = fut.IsDone()
		}
	})

	b.Run("AtomicIsDone", func(b *testing.B) {
		fut := atomic.New[int]()
		fut.Resolve(42)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = fut.IsDone()
		}
	})

	b.Run("MutexRepeatedWait", func(b *testing.B) {
		fut := New[int]()
		fut.Resolve(42)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = fut.Wait(ctx)
		}
	})

	b.Run("ChannelRepeatedWait", func(b *testing.B) {
		fut := ch.New[int]()
		fut.Resolve(42)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = fut.Wait(ctx)
		}
	})

	b.Run("AtomicRepeatedWait", func(b *testing.B) {
		fut := atomic.New[int]()
		fut.Resolve(42)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = fut.Wait(ctx)
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
