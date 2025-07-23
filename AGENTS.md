# Agent Guidelines for Future Package

## Build/Test Commands
- **Build**: `go build .` or `go build ./...`
- **Test**: `go test ./...` (all packages) or `go test .` (current package)
- **Single test**: `go test -run TestName ./package`
- **Benchmarks**: `go test -bench=. .` (main package) or `go test -bench=. ./...` (all packages)
- **Specific benchmark**: `go test -bench=BenchmarkName -benchtime=3x .`
- **GC benchmarks**: `go test -bench=BenchmarkGCPressure .`
- **Format**: `go fmt ./...`
- **Vet**: `go vet ./...`

## Code Style
- Use Go standard formatting (`go fmt`)
- Package names: lowercase, single word (e.g., `future`)
- Types: PascalCase with generics `Future[T any]`
- Functions/methods: PascalCase for exported, camelCase for unexported
- Variables: camelCase
- Constants: PascalCase for exported, camelCase for unexported

## Imports
- Standard library first, then third-party, then local packages
- Group imports with blank lines between groups
- Use full module paths for subpackages (e.g., `"github.com/metalgrid/go-future/ch"`, `"github.com/metalgrid/go-future/atomic"`)

## Error Handling
- Define package-level errors as variables (e.g., `var ErrFutureCancelled = errors.New(...)`)
- Use `errors.Is()` for error comparison
- Return zero values with errors using `var zero T; return zero, err`

## Future Implementations

### Three Available Implementations
1. **Mutex-based** (main package): Uses `sync.Mutex` + `sync.Cond`
2. **Channel-based** (`github.com/metalgrid/go-future/ch` package): Uses buffered channels + `atomic.Bool`
3. **Atomic Hybrid** (`github.com/metalgrid/go-future/atomic` package): Uses `atomic.Pointer` + signaling channel

### Performance Characteristics (100k futures)
| Implementation | Time | Memory | Allocations | Use Case |
|---------------|------|--------|-------------|----------|
| Mutex | 45ms | 19.7MB | 401k | General purpose, consistent performance |
| Channel | 47ms | 23.3MB | 500k | Familiar Go patterns, good GC behavior |
| Atomic Hybrid | **40ms** | 24.0MB | 500k | **High-throughput, best creation speed** |

### Fast Path Performance (IsDone/Wait on completed futures)
- **Mutex**: 13.1 ns/op
- **Channel**: 0.25 ns/op ⭐
- **Atomic**: 0.25 ns/op ⭐

### Pooling Benefits (reduces GC pressure by 33%)
- Use `NewFromPool[T]()` and `Put(future)` for high-frequency scenarios
- Atomic hybrid + pooling shows highest overhead currently
- Mutex + pooling most consistent for sustained workloads

### Thread Safety
- **All implementations are fully thread-safe**
- Multiple goroutines can safely call any method concurrently
- Only one Resolve/Reject call succeeds (others panic)
- Multiple Wait operations return same result

### Recommendations by Use Case
- **High-throughput async I/O**: Atomic Hybrid (fastest creation)
- **General purpose**: Mutex (most consistent)
- **Channel-heavy codebases**: Channel (familiar patterns)
- **Memory-constrained**: Mutex (lowest memory usage)
- **GC-sensitive**: Any implementation with pooling

## Concurrency
- Use `sync.Mutex` and `sync.Cond` for synchronization
- Implement context cancellation with `context.AfterFunc()`
- Use `defer` for cleanup (e.g., `defer cancel()`)
- Prefer atomic operations for lock-free fast paths