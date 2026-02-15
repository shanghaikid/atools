# Coding Patterns — Language-Agnostic Best Practices

Reference guide for common coding patterns across languages. Agents and developers can consult this for idiomatic solutions.

## Error Handling

### Wrap Errors with Context

Always add context when propagating errors up the call stack.

**Go:**
```go
if err != nil {
    return fmt.Errorf("load config %s: %w", path, err)
}
```

**Python:**
```python
try:
    result = process(data)
except ValueError as e:
    raise RuntimeError(f"failed to process {data.name}") from e
```

**TypeScript:**
```typescript
try {
    return await fetchUser(id)
} catch (error) {
    throw new Error(`failed to fetch user ${id}: ${error.message}`)
}
```

**Java:**
```java
try {
    return repository.findById(id);
} catch (DataAccessException e) {
    throw new ServiceException("failed to find user " + id, e);
}
```

### Never Swallow Errors

```
// BAD (any language): catch/except without handling
try { ... } catch (e) { }          // JS/TS
except Exception: pass              # Python
if err != nil { _ = err }          // Go (using _ to discard)

// GOOD: Handle, log, or re-throw
try { ... } catch (e) { logger.error('context', e); throw e; }
```

## Function Design

### Single Responsibility

Functions should do one thing. If you need "and" to describe it, split it.

### Early Returns

Reduce nesting by handling error/edge cases first.

```
// BAD: Deep nesting
function process(data) {
  if (data) {
    if (data.valid) {
      if (data.items.length > 0) {
        // actual logic buried here
      }
    }
  }
}

// GOOD: Early returns
function process(data) {
  if (!data) return null
  if (!data.valid) throw new Error('invalid data')
  if (data.items.length === 0) return []
  // actual logic at top level
}
```

### Accept Interfaces, Return Concrete Types

Design functions to accept the narrowest interface they need and return concrete types.

```go
// Go: Accept io.Reader, return *Result
func Process(r io.Reader) (*Result, error) { ... }
```

```python
# Python: Accept Iterable, return list
def process(items: Iterable[Item]) -> list[Result]: ...
```

```typescript
// TypeScript: Accept readonly array, return concrete type
function process(items: readonly Item[]): Result[] { ... }
```

## Concurrency

### Protect Shared State

- **Go**: Use `sync.Mutex` or channels, never bare goroutine writes
- **Python**: Use `threading.Lock` or `asyncio.Lock`
- **TypeScript**: Use atomic operations or queues for shared state
- **Java**: Use `synchronized`, `ConcurrentHashMap`, or `AtomicReference`

### Cancel Long Operations

Always support cancellation for operations that might block.

- **Go**: `context.Context` as first parameter
- **Python**: `asyncio.CancelledError` / timeout parameters
- **TypeScript**: `AbortController` / `AbortSignal`
- **Java**: `CompletableFuture` with cancellation / `InterruptedException`

### Avoid Goroutine/Thread Leaks

Every goroutine/thread/task must have a shutdown path.

## Data Structures

### Pre-allocate When Size is Known

```go
results := make([]Result, 0, len(items))  // Go
```
```python
results = [None] * len(items)  # Python (when appropriate)
```
```java
List<Result> results = new ArrayList<>(items.size());  // Java
```

### Use Builders for String Concatenation in Loops

```go
var sb strings.Builder  // Go
```
```python
parts = []; result = ''.join(parts)  # Python
```
```java
StringBuilder sb = new StringBuilder();  // Java
```
```typescript
const parts: string[] = []; parts.join('')  // TypeScript
```

## API Design

### Validate at Boundaries

Validate all external input (HTTP requests, CLI args, file reads) at the boundary. Trust internal code.

### Use Structured Errors

Return errors with machine-readable codes, not just messages.

```json
{
  "error": {
    "code": "INVALID_INPUT",
    "message": "Email format is invalid",
    "field": "email"
  }
}
```

### Pagination

Use cursor-based pagination for large datasets, not offset-based.

```
// BAD: OFFSET pagination (slow on large tables)
SELECT * FROM users ORDER BY id LIMIT 20 OFFSET 1000;

// GOOD: Cursor pagination
SELECT * FROM users WHERE id > $last_id ORDER BY id LIMIT 20;
```

## Testing Patterns

### Table-Driven Tests

Group related test cases with different inputs/expected outputs.

```go
// Go
tests := []struct{ name string; input int; want int }{
    {"positive", 5, 25},
    {"zero", 0, 0},
    {"negative", -3, 9},
}
```

```python
# Python (pytest.mark.parametrize)
@pytest.mark.parametrize("input,expected", [(5, 25), (0, 0), (-3, 9)])
def test_square(input, expected): ...
```

```typescript
// TypeScript (jest/vitest)
test.each([
    [5, 25], [0, 0], [-3, 9]
])('square(%i) = %i', (input, expected) => { ... })
```

### Mock External Dependencies

Isolate unit tests from databases, APIs, and file systems using interfaces/mocks.

## Package/Module Organization

```
project/
├── cmd/ or src/main/    # Entry points
├── internal/ or src/    # Private/business logic
├── pkg/ or lib/         # Public/shared packages
├── api/                 # API definitions (proto, OpenAPI)
├── test/ or tests/      # Test files (if not colocated)
└── docs/                # Documentation
```

## Anti-Patterns to Avoid

| Anti-Pattern | Problem | Fix |
|-------------|---------|-----|
| God Object | One class/module does everything | Split by responsibility |
| Premature Optimization | Optimizing before profiling | Profile first, optimize bottlenecks |
| Magic Numbers | `if (status === 3)` | Use named constants: `STATUS_ACTIVE` |
| Shotgun Surgery | One change requires editing many files | Better abstraction boundaries |
| Feature Envy | Function uses another module's data more than its own | Move the function |
| Mutable Global State | Package-level variables | Dependency injection |
