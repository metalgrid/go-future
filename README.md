# Go Future - Asynchronous Programming Made Simple

A high-performance, type-safe Future implementation for Go that brings JavaScript-style Promises to the Go ecosystem. Perfect for handling asynchronous operations, concurrent programming, and building responsive applications.

## What are Futures?

Futures (also known as Promises in JavaScript) represent a value that will be available at some point in the future. They're essential for:

- **Asynchronous I/O operations** (HTTP requests, database queries, file operations)
- **Concurrent task coordination** (waiting for multiple goroutines to complete)
- **Non-blocking programming** (keeping your application responsive)
- **Error handling** in async contexts (centralized error management)

## Why Use This Library?

- **🚀 High Performance**: Three optimized implementations for different use cases
- **🔒 Type Safe**: Full Go generics support with `Future[T any]`
- **♻️ Memory Efficient**: Object pooling reduces GC pressure by 33%
- **⏰ Context Aware**: Built-in timeout and cancellation support
- **🧵 Thread Safe**: Concurrent resolve/reject operations
- **📦 Zero Dependencies**: Pure Go standard library implementation

## Installation

```bash
go get github.com/metalgrid/go-future
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "github.com/metalgrid/go-future"
    "net/http"
    "time"
)

// Simulate an async HTTP request
func fetchUserData(userID int) *future.Future[string] {
    fut := future.New[string]()
    
    go func() {
        // Simulate network delay
        time.Sleep(100 * time.Millisecond)
        
        // Simulate API call
        if userID > 0 {
            fut.Resolve(fmt.Sprintf("User data for ID: %d", userID))
        } else {
            fut.Reject(fmt.Errorf("invalid user ID: %d", userID))
        }
    }()
    
    return fut
}

func main() {
    // Start async operation
    userFuture := fetchUserData(123)
    
    // Do other work while waiting...
    fmt.Println("Doing other work...")
    
    // Wait for the result with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    userData, err := userFuture.Wait(ctx)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Println(userData) // Output: User data for ID: 123
}
```

## Choose Your Implementation

This library provides three different implementations optimized for different use cases:

### 🔒 Mutex-Based (Recommended for most use cases)

**Best for**: General purpose applications, consistent performance requirements

Uses `sync.Mutex` and `sync.Cond` for synchronization. Offers the best balance of performance and memory efficiency.

```go
import "github.com/metalgrid/go-future"

// Perfect for web servers, APIs, general async operations
fut := future.New[UserData]()
go func() {
    data := fetchFromDatabase(userID)
    fut.Resolve(data)
}()
```

### 📡 Channel-Based (Familiar Go patterns)

**Best for**: Teams familiar with Go channels, excellent fast-path performance

Uses buffered channels and atomic operations. Slightly higher memory overhead but provides the fastest performance for reading already-completed futures.

```go
import "github.com/metalgrid/go-future/ch"

// Great for pipeline processing, stream handling
fut := ch.New[ProcessedData]()
go func() {
    result := processStream(inputData)
    fut.Resolve(result)
}()
```

### ⚡ Atomic Hybrid (High-throughput scenarios)

**Best for**: High-frequency future creation, maximum throughput requirements

Uses `atomic.Pointer` with signaling channels. Fastest creation speed, ideal for scenarios creating millions of futures.

```go
import "github.com/metalgrid/go-future/atomic"

// Perfect for high-frequency trading, real-time systems
fut := atomic.New[MarketData]()
go func() {
    data := getLatestPrice(symbol)
    fut.Resolve(data)
}()
```

## Real-World Examples

### HTTP Request with Timeout

```go
func fetchURL(url string) *future.Future[string] {
    fut := future.New[string]()
    
    go func() {
        resp, err := http.Get(url)
        if err != nil {
            fut.Reject(err)
            return
        }
        defer resp.Body.Close()
        
        body, err := io.ReadAll(resp.Body)
        if err != nil {
            fut.Reject(err)
            return
        }
        
        fut.Resolve(string(body))
    }()
    
    return fut
}

// Usage with timeout
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

content, err := fetchURL("https://api.example.com/data").Wait(ctx)
if err != nil {
    log.Printf("Request failed: %v", err)
}
```

### Concurrent Database Operations

```go
func getUserProfile(userID int) (*User, error) {
    // Start multiple async operations
    userFuture := fetchUser(userID)
    postsFuture := fetchUserPosts(userID)
    friendsFuture := fetchUserFriends(userID)
    
    ctx := context.Background()
    
    // Wait for all operations to complete
    user, err := userFuture.Wait(ctx)
    if err != nil {
        return nil, err
    }
    
    posts, err := postsFuture.Wait(ctx)
    if err != nil {
        return nil, err
    }
    
    friends, err := friendsFuture.Wait(ctx)
    if err != nil {
        return nil, err
    }
    
    // Combine results
    user.Posts = posts
    user.Friends = friends
    return user, nil
}
```

### High-Performance Pooling

```go
func processMany() {
    for i := 0; i < 1_000_000; i++ {
        // Use pooled futures for high-frequency operations
        fut := future.NewFromPool[int]()
        
        go func(f *future.Future[int], val int) {
            // Simulate work
            time.Sleep(time.Microsecond)
            f.Resolve(val * 2)
        }(fut, i)
        
        result, _ := fut.Wait(context.Background())
        
        // Return to pool for reuse (reduces GC pressure)
        future.Put(fut)
        
        fmt.Printf("Processed: %d\n", result)
    }
}
```

## API Reference

### Creating Futures

```go
// Standard allocation - use for most cases
fut := future.New[string]()

// From object pool - use for high-frequency operations
fut := future.NewFromPool[string]()
defer future.Put(fut) // Always return to pool
```

### Resolving Futures

```go
// Resolve with a value
fut.Resolve("success")

// Reject with an error
fut.Reject(errors.New("operation failed"))

// Note: Calling Resolve/Reject twice will panic
```

### Waiting for Results

```go
// Wait with context (recommended - supports cancellation)
result, err := fut.Wait(ctx)

// Wait with timeout (convenience method)
result, err := fut.WaitTimeout(5 * time.Second)

// Non-blocking check
if fut.IsDone() {
    result, err := fut.Wait(context.Background()) // Won't block
}
```

## Object Pooling

For high-frequency future creation, use the pooled versions to reduce GC pressure:

```go
func processMany() {
    for i := 0; i < 1000000; i++ {
        fut := future.NewFromPool[int]()
        
        go func(f *future.Future[int]) {
            f.Resolve(42)
        }(fut)
        
        result, _ := fut.Wait(context.Background())
        
        // Return to pool for reuse
        future.Put(fut)
    }
}
```

## Performance Guide

### When to Use Each Implementation

| Scenario | Recommended Implementation | Why |
|----------|---------------------------|-----|
| Web APIs, general async | **Mutex** | Best balance of performance and memory |
| High-frequency operations | **Atomic Hybrid** | Fastest creation (40ms vs 45ms for 100k futures) |
| Reading completed futures | **Channel** | Ultra-fast reads (0.25ns vs 13ns) |
| Memory-constrained | **Mutex** | Lowest memory usage (19.7MB vs 24MB) |
| Go channel fans | **Channel** | Familiar patterns, excellent GC behavior |

### Object Pooling Benefits

For high-frequency scenarios (>10k futures/second), use pooling:

```go
// Without pooling - creates garbage
for i := 0; i < 100000; i++ {
    fut := future.New[int]()
    // ... use future
} // 38% more allocations, 33% more GC cycles

// With pooling - reuses objects
for i := 0; i < 100000; i++ {
    fut := future.NewFromPool[int]()
    // ... use future
    future.Put(fut) // Return for reuse
} // Significantly less GC pressure
```

### Benchmark Results (100k futures)

| Implementation | Creation Time | Memory Usage | Best For |
|---------------|---------------|--------------|----------|
| Mutex | 45ms | 19.7MB | 🏆 General use |
| Channel | 47ms | 23.3MB | 🏆 Fast reads |
| Atomic Hybrid | **40ms** | 24.0MB | 🏆 High throughput |

*All implementations scale linearly and handle millions of concurrent operations.*

## Error Handling

The library provides clear error semantics for different failure scenarios:

```go
fut := future.New[string]()

// Reject with custom error
fut.Reject(errors.New("database connection failed"))

// Handle different error types
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()

result, err := fut.Wait(ctx)
if err != nil {
    switch {
    case errors.Is(err, future.ErrFutureCancelled):
        // Context was cancelled or timed out
        log.Println("Operation was cancelled or timed out")
    case errors.Is(err, context.DeadlineExceeded):
        // Specific timeout handling
        log.Println("Operation exceeded deadline")
    default:
        // Future was rejected with this specific error
        log.Printf("Operation failed: %v", err)
    }
}
```

### Wrapping Errors

```go
func fetchUserWithContext(userID int) *future.Future[User] {
    fut := future.New[User]()
    
    go func() {
        user, err := database.GetUser(userID)
        if err != nil {
            // Wrap errors for better context
            fut.Reject(fmt.Errorf("failed to fetch user %d: %w", userID, err))
            return
        }
        fut.Resolve(user)
    }()
    
    return fut
}
```

## Common Patterns

### Fan-Out/Fan-In Pattern

```go
func processInParallel(items []string) []string {
    futures := make([]*future.Future[string], len(items))
    
    // Fan-out: start all operations
    for i, item := range items {
        futures[i] = processItem(item)
    }
    
    // Fan-in: collect all results
    results := make([]string, len(items))
    ctx := context.Background()
    
    for i, fut := range futures {
        if result, err := fut.Wait(ctx); err == nil {
            results[i] = result
        }
    }
    
    return results
}

func processItem(item string) *future.Future[string] {
    fut := future.New[string]()
    go func() {
        // Simulate processing
        time.Sleep(100 * time.Millisecond)
        fut.Resolve(strings.ToUpper(item))
    }()
    return fut
}
```

### Circuit Breaker Pattern

```go
type CircuitBreaker struct {
    failures int
    threshold int
}

func (cb *CircuitBreaker) Call(fn func() (string, error)) *future.Future[string] {
    fut := future.New[string]()
    
    if cb.failures >= cb.threshold {
        fut.Reject(errors.New("circuit breaker open"))
        return fut
    }
    
    go func() {
        result, err := fn()
        if err != nil {
            cb.failures++
            fut.Reject(err)
        } else {
            cb.failures = 0
            fut.Resolve(result)
        }
    }()
    
    return fut
}
```

### Retry Pattern

```go
func withRetry(operation func() (string, error), maxRetries int) *future.Future[string] {
    fut := future.New[string]()
    
    go func() {
        var lastErr error
        for i := 0; i <= maxRetries; i++ {
            result, err := operation()
            if err == nil {
                fut.Resolve(result)
                return
            }
            lastErr = err
            time.Sleep(time.Duration(i) * 100 * time.Millisecond) // Exponential backoff
        }
        fut.Reject(fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr))
    }()
    
    return fut
}
```

### Timeout with Cleanup

```go
func operationWithCleanup() *future.Future[string] {
    fut := future.New[string]()
    
    go func() {
        // Setup resources
        resource := acquireResource()
        defer resource.Close() // Always cleanup
        
        // Do work
        result := doWork(resource)
        fut.Resolve(result)
    }()
    
    return fut
}

// Usage with timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := operationWithCleanup().Wait(ctx)
if errors.Is(err, future.ErrFutureCancelled) {
    log.Println("Operation timed out, but cleanup still happened")
}
```

## Testing

```bash
# Run all tests
go test ./...

# Run benchmarks (all packages)
go test -bench=. ./...

# Run specific benchmark
go test -bench=BenchmarkThreeWayComparison .

# Run GC pressure tests
go test -bench=BenchmarkGCPressure .

# Run with GC tracing
GODEBUG=gctrace=1 go test -bench=BenchmarkGCPressure .
```

## Building

```bash
# Build the project
go build ./...

# Format code
go fmt ./...

# Vet code
go vet ./...
```

## Best Practices

### ✅ Do's

```go
// Use context for cancellation and timeouts
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
result, err := fut.Wait(ctx)

// Use pooling for high-frequency operations
fut := future.NewFromPool[Data]()
defer future.Put(fut)

// Handle errors appropriately
if errors.Is(err, future.ErrFutureCancelled) {
    // Handle cancellation
}

// Use type-safe generics
fut := future.New[UserProfile]() // Not future.New[interface{}]()
```

### ❌ Don'ts

```go
// Don't call Resolve/Reject multiple times (will panic)
fut.Resolve("first")
fut.Resolve("second") // PANIC!

// Don't forget to return pooled futures
fut := future.NewFromPool[int]()
// ... use future
// future.Put(fut) // Don't forget this!

// Don't ignore context cancellation
result, _ := fut.Wait(ctx) // Should check for cancellation errors

// Don't use futures for CPU-bound work without consideration
// (Consider worker pools for CPU-intensive tasks)
```

## Migration from Other Libraries

### From JavaScript Promises

```javascript
// JavaScript
const promise = fetch('/api/data')
  .then(response => response.json())
  .catch(error => console.error(error));
```

```go
// Go Future equivalent
func fetchData() *future.Future[Data] {
    fut := future.New[Data]()
    go func() {
        resp, err := http.Get("/api/data")
        if err != nil {
            fut.Reject(err)
            return
        }
        defer resp.Body.Close()
        
        var data Data
        if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
            fut.Reject(err)
            return
        }
        
        fut.Resolve(data)
    }()
    return fut
}
```

## Contributing

We welcome contributions! Please:

1. Follow the existing code style (see `AGENTS.md`)
2. Add comprehensive tests for new functionality
3. Run benchmarks to ensure no performance regressions
4. Update documentation and examples

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

**Ready to make your Go applications more responsive?** Start with the mutex-based implementation for general use, then optimize with specific implementations as needed. Happy coding! 🚀