# Testing Patterns — Language-Agnostic Test Guide

Reference guide for writing effective tests across languages and frameworks.

## TDD Workflow

### Red-Green-Refactor

1. **RED** — Write a failing test that describes expected behavior
2. **GREEN** — Write minimal code to make the test pass
3. **REFACTOR** — Improve code quality while keeping tests green
4. **REPEAT** — Next requirement

### When to Apply TDD

- New features with clear acceptance criteria
- Bug fixes (write test that reproduces the bug first)
- Refactoring (ensure behavior preserved)

## Test Types

| Type | What to Test | Speed | When |
|------|-------------|-------|------|
| **Unit** | Individual functions in isolation | Fast (<50ms each) | Always |
| **Integration** | API endpoints, DB operations, service interactions | Medium | Always |
| **E2E** | Full user flows through the application | Slow | Critical paths only |

## Coverage Targets

| Code Type | Target |
|-----------|--------|
| Critical business logic | 90%+ |
| Public APIs | 80%+ |
| General code | 70%+ |
| Generated code / config | Exclude |

## Edge Cases You MUST Test

1. **Null/nil/undefined** — What happens with missing input?
2. **Empty** — Empty strings, arrays, maps
3. **Invalid types** — Wrong type passed (string instead of number)
4. **Boundary values** — Zero, negative, max int, very long strings
5. **Error paths** — Network failures, DB errors, permission denied
6. **Concurrent access** — Race conditions, deadlocks (if applicable)
7. **Large data** — Performance with 10k+ items
8. **Special characters** — Unicode, path separators, SQL metacharacters, newlines

## Test Structure

### Arrange-Act-Assert (AAA)

```
// 1. ARRANGE — Set up test data and dependencies
// 2. ACT — Execute the function under test
// 3. ASSERT — Verify the result
```

### Test Naming

Use descriptive names that explain the scenario:

```
TestParseConfig_ValidJSON_ReturnsConfig        // Go
test_parse_config_with_valid_json_returns_config  # Python
it('returns config when given valid JSON')      // JS/TS
parseConfig_validJson_returnsConfig()           // Java
```

## Framework Quick Reference

### Go
```bash
go test ./...                    # Run all tests
go test -v -run TestName ./pkg  # Run specific test
go test -race ./...             # Race detector
go test -cover ./...            # Coverage
go test -bench=. ./...          # Benchmarks
```

### Python
```bash
pytest                          # Run all tests
pytest -k "test_name"           # Run specific test
pytest --cov=src                # Coverage
pytest -x                       # Stop on first failure
pytest --tb=short               # Short tracebacks
```

### TypeScript/JavaScript
```bash
npm test                        # Run all tests
npx jest --watch                # Watch mode
npx jest --coverage             # Coverage
npx vitest run                  # Vitest
npx playwright test             # E2E
```

### Java
```bash
mvn test                        # Maven
gradle test                     # Gradle
mvn test -Dtest=TestName        # Specific test
```

### Rust
```bash
cargo test                      # Run all tests
cargo test test_name            # Run specific test
cargo test -- --nocapture       # Show stdout
```

## Mocking Strategies

### Interface-Based Mocking (Preferred)

Define an interface for the dependency, then create a mock implementation for tests.

```go
// Go: Interface + mock struct
type UserStore interface {
    GetUser(id string) (*User, error)
}

type MockUserStore struct {
    GetUserFunc func(id string) (*User, error)
}
func (m *MockUserStore) GetUser(id string) (*User, error) {
    return m.GetUserFunc(id)
}
```

```python
# Python: Protocol + mock
class UserStore(Protocol):
    def get_user(self, id: str) -> User: ...

# In tests: unittest.mock.MagicMock(spec=UserStore)
```

```typescript
// TypeScript: Interface + jest.fn()
interface UserStore {
    getUser(id: string): Promise<User>
}
const mockStore: UserStore = {
    getUser: jest.fn().mockResolvedValue(testUser)
}
```

### What to Mock

- External APIs and HTTP calls
- Databases and caches
- File system (for unit tests)
- Time/clock (for time-dependent logic)
- Random number generators (for deterministic tests)

### What NOT to Mock

- The code under test itself
- Simple value objects and data structures
- Standard library functions (usually)

## Test Anti-Patterns

| Anti-Pattern | Problem | Fix |
|-------------|---------|-----|
| Testing implementation | Breaks when code is refactored | Test behavior/output instead |
| Shared mutable state | Tests depend on execution order | Independent setup per test |
| Too many assertions | Hard to know what failed | One concept per test |
| Sleep/delay sync | Flaky, slow tests | Use proper waits/channels |
| Ignoring flaky tests | Erodes trust in test suite | Fix or remove immediately |
| Over-mocking | Tests pass but code is broken | Integration tests for critical paths |
| No error path tests | Only happy path covered | Test failure scenarios too |

## Test Data

### Factories/Builders

Create helper functions that produce test data with sensible defaults.

```go
func testUser(opts ...func(*User)) *User {
    u := &User{ID: "test-1", Name: "Alice", Email: "alice@test.com"}
    for _, opt := range opts {
        opt(u)
    }
    return u
}
```

```python
def make_user(**overrides):
    defaults = {"id": "test-1", "name": "Alice", "email": "alice@test.com"}
    return User(**{**defaults, **overrides})
```

### Golden Files

For complex output (rendered HTML, formatted text, serialized data), store expected output in `testdata/` files and compare.

## CI/CD Integration

```yaml
# GitHub Actions example (language-agnostic structure)
test:
  steps:
    - name: Run tests
      run: <test-command> --coverage
    - name: Check coverage threshold
      run: <coverage-check-command>
    - name: Upload coverage report
      uses: codecov/codecov-action@v4
```
