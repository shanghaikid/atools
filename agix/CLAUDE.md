# agix

Enterprise-grade LLM reverse proxy with cost tracking and shared MCP tools. Single binary, zero external dependencies.

## What it does

Sits between your AI agents and LLM providers. Transparently forwards requests, records every token, calculates cost, enforces budgets. Optionally injects shared MCP tools so all agents get tool access without any agent-side changes.

```
Your Agent → agix proxy (localhost) → OpenAI / Anthropic API
                    ↓                              ↓
              SQLite (usage log)            tool_call in response?
                    ↓                        ↓            ↓
              CLI stats / logs / export    yes            no
                                            ↓             ↓
                                      proxy executes   return to agent
                                      tool via MCP     (accumulate cost)
                                            ↓
                                      append result,
                                      loop back to LLM
```

## Architecture

```
┌─────────────────────────────────────────┐
│               CLI (cobra)                │
│  init · start · stats · logs · budget   │
│  export · tools                          │
├─────────────────────────────────────────┤
│            HTTP Proxy Server             │
│  POST /v1/chat/completions → route      │
│  GET  /v1/models           → list       │
│  GET  /health              → ok         │
├──────────┬──────────────────────────────┤
│  Router  │  gpt-* → api.openai.com     │
│          │  claude-* → api.anthropic.com│
├──────────┴──────────────────────────────┤
│  Tool Manager (optional)                 │
│  - MCP clients (stdio JSON-RPC 2.0)     │
│  - Per-agent tool filtering (allow/deny) │
│  - Tool execution loop (inject → call    │
│    LLM → execute tools → repeat)         │
├─────────────────────────────────────────┤
│  Intercept: extract usage from response  │
│  Calculate cost via pricing table        │
│  Write record to database                │
├─────────────────────────────────────────┤
│  SQLite (default) or PostgreSQL          │
│  ~/.agix/agix.db  or  postgres://...     │
└─────────────────────────────────────────┘
```

## Directory structure

```
tools/agix/
├── CLAUDE.md                          # This file
├── go.mod
├── go.sum
├── main.go                            # Entry point → cmd.Execute()
├── cmd/
│   ├── root.go                        # Root command, --config flag
│   ├── init.go                        # `agix init` - create config
│   ├── start.go                       # `agix start` - run proxy
│   ├── stats.go                       # `agix stats` - show costs
│   ├── logs.go                        # `agix logs` - request log
│   ├── budget.go                      # `agix budget` - manage budgets
│   ├── export.go                      # `agix export` - CSV/JSON
│   ├── tools.go                       # `agix tools list` - MCP tools
│   ├── doctor.go                      # `agix doctor` - health check
│   ├── audit.go                       # `agix audit list` - security events
│   ├── bundle.go                      # `agix bundle` - MCP bundles
│   ├── experiment.go                  # `agix experiment` - A/B tests
│   ├── session.go                     # `agix session` - config overrides
│   ├── trace.go                       # `agix trace` - request traces
│   └── webhook.go                     # `agix webhook` - generic webhooks
├── internal/
│   ├── config/
│   │   ├── config.go                  # YAML config read/write
│   │   └── config_test.go
│   ├── proxy/
│   │   ├── proxy.go                   # HTTP reverse proxy + tool loop
│   │   └── proxy_test.go
│   ├── store/
│   │   ├── dialect.go                 # Dialect detection + SQL rebind helpers
│   │   ├── store.go                   # Storage layer (SQLite + PostgreSQL)
│   │   └── store_test.go
│   ├── pricing/
│   │   ├── models.go                  # Model pricing table
│   │   └── models_test.go
│   ├── mcp/
│   │   ├── client.go                  # MCP client (stdio JSON-RPC 2.0)
│   │   └── client_test.go
│   ├── toolmgr/
│   │   ├── manager.go                 # Tool manager (aggregate + filter + route)
│   │   └── manager_test.go
│   ├── doctor/
│   │   ├── doctor.go                  # Health check runner + checkers
│   │   └── doctor_test.go
│   ├── ui/
│   │   ├── color.go                   # Terminal color utilities
│   │   └── color_test.go
│   ├── alert/
│   │   ├── alert.go                   # Budget alert manager
│   │   └── alert_test.go
│   ├── audit/
│   │   ├── audit.go                   # Security event logging
│   │   └── audit_test.go
│   ├── bundle/
│   │   ├── bundle.go                  # MCP server bundles
│   │   └── bundle_test.go
│   ├── cache/
│   │   ├── cache.go                   # Semantic response caching
│   │   └── cache_test.go
│   ├── compressor/
│   │   ├── compressor.go              # Context window compression
│   │   └── compressor_test.go
│   ├── dashboard/
│   │   ├── dashboard.go               # Web UI + API handlers
│   │   └── dashboard_test.go
│   ├── experiment/
│   │   ├── experiment.go              # A/B testing (traffic splitting)
│   │   └── experiment_test.go
│   ├── failover/
│   │   ├── failover.go                # Multi-provider fallback chains
│   │   └── failover_test.go
│   ├── firewall/
│   │   ├── firewall.go                # Prompt injection detection
│   │   └── firewall_test.go
│   ├── promptinject/
│   │   ├── promptinject.go            # System prompt injection
│   │   └── promptinject_test.go
│   ├── qualitygate/
│   │   ├── qualitygate.go             # Response quality checks
│   │   └── qualitygate_test.go
│   ├── ratelimit/
│   │   ├── ratelimit.go               # Per-agent rate limiting
│   │   └── ratelimit_test.go
│   ├── responsepolicy/
│   │   ├── responsepolicy.go          # Response post-processing
│   │   └── responsepolicy_test.go
│   ├── router/
│   │   ├── router.go                  # Smart model routing
│   │   └── router_test.go
│   ├── session/
│   │   ├── session.go                 # Session config overrides
│   │   └── session_test.go
│   ├── trace/
│   │   ├── trace.go                   # Request tracing + spans
│   │   └── trace_test.go
│   ├── webhook/
│   │   ├── webhook.go                 # Generic webhook handler
│   │   └── webhook_test.go
```

## Tech stack

| Component | Choice | Reason |
|-----------|--------|--------|
| Language | Go 1.26 | Single binary, cross-compile, great for network I/O |
| CLI | github.com/spf13/cobra | Industry standard (Docker, K8s, gh CLI) |
| Database | modernc.org/sqlite (default) + github.com/lib/pq (optional) | SQLite for single-machine, PostgreSQL for scalable deployments |
| Tables | github.com/olekukonko/tablewriter | Terminal table formatting |
| Config | gopkg.in/yaml.v3 | YAML config parsing |

Zero CGO means `GOOS=linux GOARCH=amd64 go build` just works.

## Data model

Single table, all fields non-null:

```sql
CREATE TABLE IF NOT EXISTS requests (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp     DATETIME NOT NULL DEFAULT (datetime('now')),
    agent_name    TEXT NOT NULL DEFAULT '',
    model         TEXT NOT NULL,
    provider      TEXT NOT NULL,
    input_tokens  INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cost_usd      REAL NOT NULL DEFAULT 0,
    duration_ms   INTEGER NOT NULL DEFAULT 0,
    status_code   INTEGER NOT NULL DEFAULT 200
);
```

Indexes on `timestamp`, `agent_name`, `model`. Timestamps stored as ISO 8601 strings (`2006-01-02T15:04:05Z`) for SQLite date function compatibility.

### Database backends

- **SQLite (default)**: `database: "~/.agix/agix.db"` — single file, zero setup
- **PostgreSQL (optional)**: `database: "postgres://user:pass@localhost:5432/agix?sslmode=disable"` — scalable, multi-machine

Detection is automatic: if the `database` value starts with `postgres://` or `postgresql://`, the PostgreSQL driver is used. The `Dialect` type in `internal/store/dialect.go` provides helpers for SQL differences (`Rebind` for placeholder rewriting, dialect-specific DDL).

### Prompt Template Injection (`internal/promptinject/`)

Injects system prompts into all requests at the proxy level. Supports global templates (applied to all agents) and per-agent overrides. Templates can be prepended or appended to existing system messages.

- **Position**: After firewall scan, before cache lookup
- **Format**: Operates on OpenAI-compatible format only (before Anthropic conversion)
- **Config**: `prompt_templates.enabled`, `.global`, `.agents`, `.position` (prepend/append)

## Proxy behavior

### Request flow (without tools)

1. Agent sends `POST /v1/chat/completions` (OpenAI-compatible format)
2. Proxy reads `model` field to determine provider
3. For Anthropic models: converts OpenAI format → Anthropic Messages API format
4. Injects real API key from config (agent never sees the key)
5. Forwards to upstream provider
6. Intercepts response, extracts `usage` (prompt_tokens, completion_tokens)
7. Calculates cost from pricing table
8. Writes record to SQLite
9. Returns original response to agent (adds `X-Cost-USD`, `X-Input-Tokens`, `X-Output-Tokens` headers)

### Request flow (with tools)

When MCP servers are configured and the agent has available tools:

1. Agent sends request (same OpenAI-compatible format, unaware of tools)
2. Proxy forces `stream: false` (tool loop requires non-streaming)
3. Proxy injects tool definitions into request body
4. Sends to upstream LLM
5. If LLM returns `tool_calls`:
   - Proxy routes each call to the appropriate MCP server
   - Executes tool, collects results
   - Appends assistant message + tool results to conversation
   - Loops back to step 4 (up to `max_iterations`)
6. When LLM returns without `tool_calls`:
   - Strips tool-related fields from response
   - Returns clean response to agent (agent never sees tools)
   - Records accumulated tokens/cost as a single store entry

### Tool call format by provider

- **OpenAI**: `finish_reason: "tool_calls"` + `message.tool_calls[]` → results via `role: "tool"` messages
- **Anthropic**: `stop_reason: "tool_use"` + `content[].type: "tool_use"` → results via `tool_result` content blocks

### Streaming (SSE)

For `"stream": true` requests without tools, the proxy:
- Forwards each SSE chunk to the client immediately (via `http.Flusher`)
- Parses each `data:` line looking for usage info
- OpenAI sends usage in the final chunk; Anthropic sends in `message_delta` or `message_stop` events
- Records totals after stream completes

Note: When tools are active, streaming is forced to non-streaming. The agent sees a normal non-streaming response.

### Agent identification

Agents identify themselves via `X-Agent-Name` header. This enables per-agent stats, budget enforcement, and tool access control. If omitted, recorded as empty string (displayed as "(unknown)" in stats).

### Budget enforcement

When an agent has a configured budget and sends a request:
- Proxy checks current daily/monthly spend against limits
- If over budget: returns `429 Too Many Requests` with error message
- If check fails (DB error): allows request (fail-open)

## Config file

Location: `~/.agix/config.yaml`

```yaml
port: 8080
keys:
  openai: "sk-..."
  anthropic: "sk-ant-..."
database: "/Users/you/.agix/agix.db"  # or "postgres://user:pass@localhost:5432/agix?sslmode=disable"
log_level: info
budgets:
  code-reviewer:
    daily_limit_usd: 10.0
    monthly_limit_usd: 200.0
    alert_at_percent: 80
tools:
  max_iterations: 10          # max tool execution rounds per request
  servers:
    filesystem:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    github:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-github"]
      env: ["GITHUB_TOKEN=ghp_xxx"]
  agents:
    code-reviewer:
      allow: ["read_file", "list_directory"]    # whitelist
    docs-writer:
      deny: ["write_file", "delete_file"]       # blacklist
    # agents not listed get all tools
```

Created by `agix init`. File permissions: `0600`. Directory permissions: `0700`.

## MCP protocol

The proxy communicates with MCP servers via stdio JSON-RPC 2.0:

- **Transport**: `exec.Command` spawns child process, communicates via stdin/stdout
- **Message format**: one JSON-RPC message per line, `\n` delimited
- **Handshake**: `initialize` → response → `notifications/initialized` notification
- **Discovery**: `tools/list` returns available tools
- **Execution**: `tools/call` with tool name and arguments
- **Shutdown**: close stdin → SIGINT → `cmd.Wait()`
- **Concurrency**: all calls serialized via mutex (single stdin/stdout pair per server)

## CLI commands

```bash
# Core
agix init                          # Create config with defaults
agix start [--port 8080]           # Start proxy
agix doctor                        # Check config and dependencies

# Statistics
agix stats                         # Overall stats (today)
agix stats --group-by agent        # Per-agent breakdown
agix stats --group-by model        # Per-model breakdown
agix stats --group-by day          # Daily costs
agix stats --period 2026-01        # Specific month

# Logs
agix logs                          # Recent 20 requests
agix logs --tail                   # Live tail (poll 500ms)
agix logs --agent code-reviewer    # Filter by agent
agix logs -n 100                   # Last 100 requests

# Budget
agix budget list                   # Show all budgets
agix budget set <agent> [flags]    # Set budget
agix budget remove <agent>         # Remove budget

# Export
agix export --format csv           # Export CSV
agix export --format json          # Export JSON
agix export --period 2026-01       # Specific month

# Tools
agix tools list                    # List all MCP tools
agix bundle list                   # List MCP bundles
agix bundle install <name>         # Install a bundle
agix bundle remove <name>          # Remove a bundle

# Observability
agix audit list                    # Security events
agix audit list --type tool_call   # Filter by type
agix trace list                    # Recent request traces
agix trace <trace-id>              # Show detailed trace
agix experiment list               # List A/B tests
agix experiment check <agent> <model>  # Check variant assignment

# Session & Webhooks
agix session list                  # Active session overrides
agix session clean                 # Clean expired overrides
agix webhook list                  # Configured webhooks
agix webhook history               # Webhook execution history
```

## HTTP API Reference

### Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/chat/completions` | POST | Proxied LLM chat completions (OpenAI-compatible) |
| `/v1/models` | GET | List available models and pricing |
| `/v1/sessions/{id}` | GET/POST | Get/set session config overrides |
| `/v1/webhooks/{name}` | POST | Receive webhook (HMAC-SHA256 verified) |
| `/health` | GET | Health check endpoint |
| `/dashboard/` | GET | Web dashboard (if enabled) |
| `/api/stats` | GET | Aggregated usage statistics |
| `/api/agents` | GET | Per-agent statistics |
| `/api/budgets` | GET | Budget info and current spend |
| `/api/costs/daily` | GET | Daily costs (last 30 days) |
| `/api/logs` | GET | Recent request logs |

### Request headers

- `X-Agent-Name` — agent identifier (enables per-agent tracking, budgets, tool access)
- `X-Session-ID` — session ID for per-session config overrides
- `X-Webhook-Signature` — HMAC-SHA256 signature for webhook verification (format: `sha256=HEX`)

### Response headers

- `X-Cost-USD` — calculated cost for this request
- `X-Input-Tokens` — prompt tokens used
- `X-Output-Tokens` — completion tokens generated
- `X-Trace-ID` — request trace ID for observability
- `X-Cache` — "HIT" if response from semantic cache, "MISS" otherwise
- `X-Firewall-Warning` — warnings from prompt firewall (if any matched rules)
- `X-Quality-Warning` — quality gate issues (empty response, truncated, refusal)
- `X-Response-Policy` — redaction rules applied (e.g., "email_mask, truncated")
- `Retry-After` — seconds to wait if rate limited (429 response code)

### Status codes

- `200` — request processed successfully
- `429` — rate limited or budget exceeded (check `Retry-After` header)
- `500` — upstream provider error or server error
- `503` — database error (fail-open, request still forwarded)

## Health checks (agix doctor)

The `agix doctor` command performs comprehensive health checks to verify the gateway is configured and ready:

**Check: Config File Permissions**
- Verifies config file has restrictive permissions (0600: owner read/write only)
- Warns if readable by group or others (contains sensitive API keys)
- Pass: permissions OK, Warn: overly permissive, Fail: never (warning-only)

**Check: API Key Validity**
- Makes HTTPS requests to each provider's endpoint with configured API keys
- Tests OpenAI, Anthropic, and DeepSeek providers
- Uses correct auth headers: `Authorization: Bearer {key}` (OpenAI/DeepSeek), `x-api-key` header (Anthropic)
- Pass: all configured keys valid, Warn: no keys configured, Fail: invalid keys (401/403)

**Check: Budget Configuration**
- Validates budget rules make logical sense
- Ensures daily limit ≤ monthly limit
- Ensures alert_at_percent is in range [1, 100]
- Pass: config valid, Warn: inconsistencies detected, Fail: never (warning-only)

**Check: Firewall Rules**
- Compiles each custom regex pattern (catches syntax errors)
- Validates action field is one of: "block", "warn", "log"
- Pass: all rules valid, Fail: invalid regex or unknown action

**Check: Database Connectivity & Integrity**
- Auto-detects database type (SQLite vs PostgreSQL)
- For SQLite: checks file exists, runs `PRAGMA integrity_check`, verifies result is "ok"
- For PostgreSQL: opens connection (5-second timeout), runs `SELECT version()`, verifies success
- Pass: database healthy, Warn: SQLite file doesn't exist yet (will be created), Fail: connection/integrity issues

**Exit code**
- `0` if all checks pass
- `N` (count of failed checks) if any checks fail

## User integration

Users change one line to route through the proxy:

```python
# OpenAI SDK
client = OpenAI(base_url="http://localhost:8080/v1", api_key="unused")

# Or via environment variable (zero code change)
export OPENAI_BASE_URL=http://localhost:8080/v1
```

Optional agent identification:
```python
client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="unused",
    default_headers={"X-Agent-Name": "my-agent"},
)
```

Tool access is transparent — agents don't need any code changes to use shared tools. The proxy handles injection, execution, and response cleanup.

## Code conventions

- **Go standard layout**: `cmd/` for CLI, `internal/` for private packages
- **Error wrapping**: always `fmt.Errorf("context: %w", err)`
- **No globals**: config and store passed explicitly via struct fields
- **Table-driven tests**: all test files use `[]struct{ name string; ... }` pattern
- **No CGO**: pure Go only, for cross-compilation
- **Fail-open on non-critical errors**: budget check DB failure allows request through
- **ISO 8601 timestamps**: all times stored as `2006-01-02T15:04:05Z` strings in SQLite

## Build & test

```bash
cd tools/agix

# Build
go build -o agix .

# Test
go test ./...

# Test with verbose
go test -v ./...

# Cross compile
GOOS=linux GOARCH=amd64 go build -o agix-linux .
GOOS=darwin GOARCH=arm64 go build -o agix-darwin .

# Vet
go vet ./...
```

## Supported models

### OpenAI
gpt-5.2, gpt-5.1, gpt-5, gpt-5-mini, gpt-5-nano, gpt-4.1, gpt-4.1-mini, gpt-4.1-nano, gpt-4o, gpt-4o-mini, o1, o3, o3-mini, o4-mini

### Anthropic
claude-opus-4-6, claude-opus-4-5-20251101, claude-opus-4-1-20250805, claude-opus-4-20250514, claude-sonnet-4-5-20250929, claude-sonnet-4-20250514, claude-haiku-4-5-20251001, claude-3-5-haiku-20241022, claude-3-haiku-20240307

Versioned model names (e.g., `gpt-4o-2024-08-06`) are matched via longest-prefix against the pricing table.

## Enterprise considerations

- **API key isolation**: agents never see real API keys; proxy injects them
- **Audit trail**: every LLM call recorded with timestamp, agent, model, tokens, cost, latency, status
- **Budget enforcement**: per-agent daily/monthly limits, enforced at proxy level
- **Shared tools**: centralized MCP servers, per-agent access control (allow/deny lists)
- **Data locality**: all data in a local SQLite file, nothing leaves the machine
- **Extensibility**: add new providers by adding a case to `buildUpstreamRequest` and entries to the pricing table
- **NO_COLOR support**: respects `NO_COLOR` env var for CI/pipeline environments

## Implemented features

All planned features are now implemented:
- ✅ Budget alert webhooks (via alert package)
- ✅ Web dashboard (via dashboard package, enabled in config)
- ✅ Semantic response caching (via cache package)
- ✅ Multi-provider failover with fallback chains (via failover package)
- ✅ Per-agent rate limiting (via ratelimit package)
- ✅ Audit logging with optional content capture (via audit package)
- ✅ Prompt firewall injection detection (via firewall package)
- ✅ Smart model routing by request complexity (via router package)
- ✅ Context compression for long conversations (via compressor package)
- ✅ A/B testing with traffic splitting (via experiment package)
- ✅ Response policy with redaction/truncation (via responsepolicy package)
- ✅ Quality gate for response validation (via qualitygate package)
- ✅ Request tracing and observability spans (via trace package)
- ✅ Session-level config overrides (via session package)
- ✅ Generic webhook handling (via webhook package)
- ✅ System prompt injection (via promptinject package)
- ✅ MCP tool bundles (via bundle package)
