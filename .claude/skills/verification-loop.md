# Verification Loop — Pre-PR Quality Gates

A systematic verification process to run before creating a pull request or marking a story as done.

## When to Use

- After completing a feature or significant code change
- Before the reviewer and tester agents are dispatched
- After rework cycles to ensure fixes don't introduce regressions

## Verification Phases

### Phase 1: Build Verification

Run the project's build command. If it fails, stop and fix before continuing.

| Project Type | Command |
|-------------|---------|
| Go | `go build ./...` |
| Node.js/TypeScript | `npm run build` or `npx tsc --noEmit` |
| Python | `python -m py_compile` or `mypy .` |
| Rust | `cargo build` |
| Java (Maven) | `mvn compile` |
| Java (Gradle) | `gradle build` |
| Makefile | `make build` |

### Phase 2: Static Analysis / Lint

| Project Type | Command |
|-------------|---------|
| Go | `go vet ./...` then `staticcheck ./...` (if available) |
| Node.js/TypeScript | `npm run lint` or `npx eslint .` |
| Python | `ruff check .` or `pylint` |
| Rust | `cargo clippy` |
| Java | `mvn checkstyle:check` or IDE warnings |

### Phase 3: Test Suite

Run the full test suite with coverage:

| Project Type | Command |
|-------------|---------|
| Go | `go test -cover ./...` |
| Node.js | `npm test -- --coverage` |
| Python | `pytest --cov=src` |
| Rust | `cargo test` |
| Java | `mvn test` |

Report:
- Total tests: N
- Passed: N
- Failed: N
- Coverage: N%

### Phase 4: Security Quick Scan

Search for common security issues in changed files:

```bash
# Hardcoded secrets
git diff main...HEAD | grep -iE '(api_key|apikey|secret|password|token|private_key).*=.*["'"'"']'

# Debug statements
git diff main...HEAD | grep -iE '(console\.log|print\(|fmt\.Print|System\.out\.print|debug!)'

# Unsafe patterns
git diff main...HEAD | grep -iE '(eval\(|exec\(|innerHTML|dangerouslySetInnerHTML|InsecureSkipVerify|shell=True)'
```

### Phase 5: Diff Review

```bash
# Summary of changes
git diff --stat main...HEAD

# List changed files
git diff --name-only main...HEAD
```

Review each changed file for:
- Unintended changes (debugging code, commented-out code)
- Missing error handling
- Potential edge cases not covered by tests

## Verification Report Format

```
VERIFICATION REPORT
==================

Build:     [PASS/FAIL]
Lint:      [PASS/FAIL] (N warnings)
Tests:     [PASS/FAIL] (N/M passed, N% coverage)
Security:  [PASS/FAIL] (N issues)
Diff:      [N files changed, +A/-D lines]

Overall:   [READY/NOT READY] for review

Issues to Fix:
1. ...
2. ...
```

## Decision Guide

| Result | Action |
|--------|--------|
| All PASS | Proceed to review/testing phase |
| Build FAIL | Fix immediately, do not proceed |
| Tests FAIL | Fix failing tests before proceeding |
| Security FAIL | Fix critical issues, document accepted risks |
| Lint warnings only | Proceed, note in review |
