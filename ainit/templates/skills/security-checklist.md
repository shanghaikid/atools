# Security Checklist — Pre-Merge Security Review

Reference checklist for security review before merging code changes.

## Secrets Management

- [ ] No hardcoded API keys, passwords, tokens, or connection strings in source
- [ ] All secrets loaded from environment variables or secret managers
- [ ] `.env` / `.env.local` files in `.gitignore`
- [ ] No secrets in git history (if found, rotate immediately)
- [ ] Example env file (`.env.example`) contains placeholder values only

## Input Validation

- [ ] All user input validated with schemas before processing
- [ ] File uploads restricted (size, type, extension)
- [ ] No direct use of user input in queries (use parameterized queries)
- [ ] Whitelist validation preferred over blacklist
- [ ] Error messages don't leak sensitive information or stack traces

## Injection Prevention

### SQL Injection
```
BAD:  query = f"SELECT * FROM users WHERE id = {user_id}"
GOOD: query = "SELECT * FROM users WHERE id = $1", [user_id]
```

### Command Injection
```
BAD:  os.system(f"convert {user_filename}")
GOOD: subprocess.run(["convert", user_filename])  # list form, no shell
```

### Path Traversal
```
BAD:  open(f"/uploads/{user_path}")
GOOD: safe_path = os.path.normpath(user_path)
      if ".." in safe_path: raise Error("invalid path")
      full = os.path.join("/uploads", safe_path)
      if not full.startswith("/uploads/"): raise Error("path escape")
```

### XSS
```
BAD:  innerHTML = userInput
GOOD: textContent = userInput  // or sanitize with DOMPurify
```

## Authentication & Authorization

- [ ] Passwords hashed with strong algorithm (bcrypt, argon2, scrypt)
- [ ] JWT tokens validated (signature, expiry, issuer)
- [ ] Authorization checked on every protected endpoint
- [ ] Session tokens regenerated after authentication
- [ ] No credentials in URL parameters
- [ ] Rate limiting on authentication endpoints

## API Security

- [ ] CORS properly configured (not wildcard `*` for authenticated APIs)
- [ ] Rate limiting on all public endpoints
- [ ] Stricter rate limits on expensive operations (search, AI calls)
- [ ] Request body size limits configured
- [ ] Timeout configured for external HTTP calls
- [ ] No internal error details exposed to clients

## Data Protection

- [ ] HTTPS enforced in production
- [ ] Sensitive data encrypted at rest
- [ ] PII handling compliant with requirements
- [ ] No sensitive data in logs (passwords, tokens, credit cards)
- [ ] Proper data retention and deletion policies

## Dependencies

- [ ] No packages with known critical CVEs
- [ ] Lock files committed (package-lock.json, go.sum, Pipfile.lock, Cargo.lock)
- [ ] Dependencies from trusted sources only
- [ ] Regular dependency updates scheduled

## Security Headers (Web Applications)

```
Content-Security-Policy: default-src 'self'
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Strict-Transport-Security: max-age=31536000; includeSubDomains
Referrer-Policy: strict-origin-when-cross-origin
```

## Language-Specific Risks

### Go
- [ ] No `InsecureSkipVerify: true` in production TLS config
- [ ] No `unsafe` package without justification
- [ ] Race conditions checked (`go test -race`)
- [ ] `os/exec` not used with unsanitized user input

### Python
- [ ] No `eval()`, `exec()`, `pickle.loads()` on untrusted data
- [ ] `yaml.safe_load()` used instead of `yaml.load()`
- [ ] `subprocess` uses list form, not `shell=True` with user input
- [ ] No bare `except:` that hides security errors

### TypeScript/JavaScript
- [ ] No `eval()` or `Function()` constructor with user data
- [ ] No `dangerouslySetInnerHTML` with unsanitized content
- [ ] Sensitive tokens in httpOnly cookies, not localStorage
- [ ] CSRF protection on state-changing endpoints

### Java
- [ ] XML parsers disable external entities (prevent XXE)
- [ ] `PreparedStatement` used for SQL, not string concatenation
- [ ] `SecureRandom` used for security-sensitive randomness
- [ ] Deserialization of untrusted data avoided

### Rust
- [ ] `unsafe` blocks minimized and documented
- [ ] No `unwrap()` on user-controlled data in production
- [ ] Dependencies audited with `cargo audit`

## Incident Response

If a CRITICAL vulnerability is found:
1. Document the vulnerability with exact location and impact
2. Flag as `request_changes` immediately
3. Provide the secure code pattern as a fix suggestion
4. If credentials were exposed, note that rotation is required
