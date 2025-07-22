# Agent Guidelines for Future Package

## Build/Test Commands
- **Build**: `go build .` or `go build ./...`
- **Test**: `go test ./...` (all packages) or `go test .` (current package)
- **Single test**: `go test -run TestName ./package`
- **Run**: `go run main.go`
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
- Use relative imports for local packages (e.g., `"future/future"`)

## Error Handling
- Define package-level errors as variables (e.g., `var ErrFutureCancelled = errors.New(...)`)
- Use `errors.Is()` for error comparison
- Return zero values with errors using `var zero T; return zero, err`

## Concurrency
- Use `sync.Mutex` and `sync.Cond` for synchronization
- Implement context cancellation with `context.AfterFunc()`
- Use `defer` for cleanup (e.g., `defer cancel()`)