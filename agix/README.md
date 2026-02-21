# agix

Enterprise-grade LLM reverse proxy with cost tracking, budget enforcement, and shared MCP tools. Single binary, zero external dependencies.

```
Agent → agix (localhost:8080) → OpenAI / Anthropic / DeepSeek
             ↓
        SQLite/PostgreSQL
             ↓
    stats · logs · budget · export · doctor · audit · trace · webhook
```

## Features

**Core:**
- **Cost tracking** — records tokens, calculates cost for every request, stores in SQLite or PostgreSQL
- **Budget enforcement** — per-agent daily/monthly limits, returns 429 when exceeded
- **Multi-provider** — routes `gpt-*` to OpenAI, `claude-*` to Anthropic, `deepseek-*` to DeepSeek, auto-converts between API formats
- **API key isolation** — agents never see real API keys, proxy injects them
- **Streaming support** — transparent SSE pass-through with usage extraction
- **Single binary, zero CGO** — pure Go, cross-compiles to any platform

**Tools & MCP:**
- **Shared MCP tools** — inject tools from MCP servers into LLM conversations transparently
- **Tool bundles** — pre-packaged MCP server sets (install with one command)
- **Tool access control** — per-agent allow/deny lists for tool discovery

**Intelligence & Optimization:**
- **Smart routing** — automatically routes simple requests to cheaper models (cost optimization)
- **Semantic caching** — embedding-based response caching to reduce LLM calls for similar prompts
- **Context compression** — automatically summarizes old messages when conversations get long
- **A/B testing** — deterministic traffic splitting for model experiments

**Safety & Control:**
- **Prompt firewall** — detects and blocks prompt injection attempts, PII, and policy violations
- **Response policy** — redacts sensitive patterns, enforces output format, truncates responses
- **Quality gate** — detects empty/truncated/refused responses and auto-retries
- **Session overrides** — per-session config (model, temperature, max_tokens) with TTL expiration

**Observability:**
- **Request tracing** — detailed per-request spans showing all pipeline steps and timing
- **Audit logging** — security event log (tool calls, firewall blocks, content) with optional content capture
- **Metrics dashboard** — web dashboard with cost visualization and budget monitoring
- **Doctor command** — health check for config, API keys, database, MCP servers

**Reliability & Scale:**
- **Multi-provider failover** — automatic fallback chains on provider errors
- **Rate limiting** — per-agent request throttling (requests/min and requests/hour)
- **Budget alerts** — webhook notifications when spending hits thresholds
- **Webhook endpoints** — generic webhook support with template rendering and LLM execution

## Quick start

```bash
# Build
make build

# Initialize config
./agix init

# Add your API keys
vim ~/.agix/config.yaml

# Health check
./agix doctor

# Start the proxy
./agix start
```

Point your agents to the proxy:

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="unused",  # proxy injects the real key
    default_headers={"X-Agent-Name": "my-agent"},
)

resp = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Hello"}],
)
```

Or via environment variable (zero code change):

```bash
export OPENAI_BASE_URL=http://localhost:8080/v1
```

## Install

```bash
make build          # Build binary
make install        # Install to /usr/local/bin
make test           # Run tests
make clean          # Remove binary
```

Cross-compile:

```bash
GOOS=linux GOARCH=amd64 go build -o agix .
GOOS=darwin GOARCH=arm64 go build -o agix .
```

## Configuration

Config lives at `~/.agix/config.yaml` (created by `agix init`):

```yaml
port: 8080
log_level: info

keys:
  openai: "sk-..."
  anthropic: "sk-ant-..."
  deepseek: "sk-..."

database: "/Users/you/.agix/agix.db"  # or "postgres://user:pass@localhost/agix"

# Per-agent spending limits
budgets:
  code-reviewer:
    daily_limit_usd: 10.0
    monthly_limit_usd: 200.0
    alert_at_percent: 80           # Alert when 80% spent

# Shared MCP tools
tools:
  max_iterations: 10               # Max tool execution rounds
  servers:
    filesystem:
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    github:
      command: npx
      args: ["-y", "@modelcontextprotocol/server-github"]
      env: ["GITHUB_TOKEN=ghp_xxx"]
  agents:
    code-reviewer:
      allow: ["read_file", "list_directory"]    # whitelist
    docs-writer:
      deny: ["write_file"]                       # blacklist

# Smart routing (cost optimization)
routing:
  enabled: true
  tiers:
    simple:
      max_message_tokens: 500      # Classify as simple
      max_messages: 3
  model_map:
    gpt-4o:
      simple: "gpt-4o-mini"        # Route simple → mini
      complex: "gpt-4o"

# Semantic caching (reduce LLM calls)
cache:
  enabled: true
  similarity_threshold: 0.95       # Cosine similarity threshold
  ttl_minutes: 60

# Context compression (handle long conversations)
compression:
  enabled: true
  threshold_tokens: 50000          # Trigger at this token count
  keep_recent: 10                  # Keep N recent messages uncompressed
  summary_model: "gpt-4o-mini"

# Prompt firewall (block injections)
firewall:
  enabled: true
  # Built-in rules for injection, PII, etc.
  rules:
    - name: "custom_rule"
      pattern: "(?i)ignore.*previous"
      action: "block"              # block, warn, or log

# Response policy (redaction, truncation)
response_policy:
  enabled: true
  max_output_chars: 5000           # Truncate at this length
  redact_patterns:
    - name: "email_mask"
      pattern: "[A-Z0-9._%+-]+@[A-Z0-9.-]+"
      replacement: "[EMAIL]"
  agents:
    sensitive-agent:
      max_output_chars: 1000       # Per-agent override

# Quality gate (auto-retry on issues)
quality_gate:
  enabled: true
  max_retries: 2
  on_empty: "retry"                # retry, warn, or reject
  on_truncated: "warn"
  on_refusal: "warn"

# A/B testing (traffic splitting)
experiments:
  - name: "test-gpt4o-vs-mini"
    enabled: true
    control_model: "gpt-4o"
    variant_model: "gpt-4o-mini"
    traffic_pct: 20                # 20% to variant, 80% to control

# Multi-provider failover
failover:
  max_retries: 2
  chains:
    gpt-4o: ["gpt-4o-mini", "gpt-35-turbo"]
    claude-opus-4-6: ["claude-sonnet-4-5-20250929"]

# Rate limiting (per-agent)
rate_limits:
  expensive-agent:
    requests_per_minute: 10
    requests_per_hour: 100

# System prompt injection
prompt_templates:
  enabled: true
  global: "You are a helpful assistant."
  agents:
    code-reviewer: "Review code for security issues."
  position: "prepend"              # or "append"

# Session-level config overrides
session_overrides:
  enabled: true
  default_ttl: "1h"                # Session lifetime

# Request tracing & spans
tracing:
  enabled: true
  sample_rate: 1.0                 # 0-1, log all by default

# Security audit logging
audit:
  enabled: true
  content_log: true                # Log full request/response bodies
  dangerous_tools: ["delete_file", "execute_cmd"]

# Web dashboard
dashboard:
  enabled: true                    # Serves at /dashboard/

# Generic webhooks
webhooks:
  enabled: true
  definitions:
    summarize-report:
      secret: "webhook_secret"
      model: "gpt-4o-mini"
      prompt_template: "Summarize: {{.Payload}}"
      callback_url: "https://api.example.com/callback"

# Bundled MCP server sets
bundles: ["basic"]                 # Install built-in or user bundles
```

## CLI

### Core commands

```bash
agix init                          # Create config with defaults
agix start [--port 8080]           # Start proxy server
agix doctor                        # Health check (config, keys, database, MCP servers)
```

### Statistics & monitoring

```bash
agix stats                         # Today's overall stats
agix stats --period 7d             # Last 7 days
agix stats --period 2026-01        # Specific month (YYYY-MM)
agix stats --group-by agent        # Per-agent breakdown
agix stats --group-by model        # Per-model breakdown
agix stats --group-by day          # Daily costs (graph-friendly)
agix stats --format json           # JSON output

agix logs                          # Last 20 requests
agix logs -n 100                   # Last 100 requests
agix logs --tail                   # Watch in real-time (poll every 500ms)
agix logs --agent my-agent         # Filter by agent
```

### Budget management

```bash
agix budget                        # Show all budgets + current spend
agix budget set -a agent -d 5.00   # Set daily limit
agix budget set -a agent -m 100    # Set monthly limit
agix budget remove -a agent        # Remove budget
```

### Data export

```bash
agix export                        # CSV to stdout
agix export --format json          # JSON to stdout
agix export -o costs.csv           # CSV to file
agix export --period 30d           # Specific period
```

### MCP tools

```bash
agix tools list                    # List discovered MCP tools
agix bundle list                   # List available bundles
agix bundle show <name>            # Show bundle details
agix bundle install <name>         # Install a bundle
agix bundle remove <name>          # Remove a bundle
```

### Observability

```bash
agix audit list                    # Show security events
agix audit list --type tool_call   # Filter by event type
agix audit list --agent reviewer   # Filter by agent
agix audit list -n 50              # Show 50 events

agix trace list                    # Show recent request traces
agix trace <trace-id>              # Show detailed trace with spans
agix trace list --agent reviewer   # Filter by agent
```

### Experiments & features

```bash
agix experiment list               # List configured A/B tests
agix experiment check agent gpt-4o # Check which variant for agent

agix session list                  # List active session overrides
agix session clean                 # Clean expired overrides

agix webhook list                  # List configured webhooks
agix webhook history               # Show webhook execution history
```

## HTTP API

### Request/response headers

**Request headers:**
- `X-Agent-Name` — agent identifier (enables per-agent stats, budgets, tools)
- `X-Session-ID` — session ID for per-session config overrides

**Response headers:**
- `X-Cost-USD` — calculated cost for this request
- `X-Input-Tokens` — prompt tokens used
- `X-Output-Tokens` — completion tokens generated
- `X-Trace-ID` — request trace ID (for observability)
- `X-Cache` — "HIT" if response from semantic cache, "MISS" otherwise
- `X-Firewall-Warning` — warnings from prompt firewall (if any)
- `X-Quality-Warning` — quality gate issues detected (if any)
- `X-Response-Policy` — redaction rules applied (if any)
- `Retry-After` — seconds to wait if rate limited (429 response)

### Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/chat/completions` | POST | Proxied LLM chat completions |
| `/v1/models` | GET | List available models |
| `/v1/sessions/{session-id}` | GET/POST | Manage session config overrides |
| `/v1/webhooks/{name}` | POST | Webhook endpoint (HMAC-SHA256 verified) |
| `/health` | GET | Health check (returns 200 OK) |
| `/dashboard/` | GET | Web dashboard (if enabled) |
| `/api/stats` | GET | API: aggregated statistics |
| `/api/agents` | GET | API: per-agent statistics |
| `/api/budgets` | GET | API: budget info and spend |
| `/api/costs/daily` | GET | API: daily costs (last 30 days) |
| `/api/logs` | GET | API: recent requests |

### Supported models

**OpenAI:** gpt-5.2, gpt-5.1, gpt-5, gpt-5-mini, gpt-5-nano, gpt-4.1, gpt-4.1-mini, gpt-4.1-nano, gpt-4o, gpt-4o-mini, o1, o3, o3-mini, o4-mini

**Anthropic:** claude-opus-4-6, claude-opus-4-5-20251101, claude-sonnet-4-5-20250929, claude-haiku-4-5-20251001, claude-3-5-haiku-20241022

**DeepSeek:** deepseek-chat, deepseek-reasoner

Versioned model names (e.g., `gpt-4o-2024-08-06`) are matched via longest-prefix against the pricing table.

## How it works

### Request flow (without tools)

1. Agent sends `POST /v1/chat/completions` with optional `X-Agent-Name` header
2. Proxy validates budget (if configured) — returns 429 if over limit
3. Proxy checks rate limit (if configured) — returns 429 if exceeded
4. Firewall scans user messages for injections/PII — blocks or warns
5. Prompt templates injected (if configured)
6. Semantic cache lookup (if enabled) — returns cached response if high similarity
7. Router determines if request is simple/complex (if routing enabled)
8. Reads `model` field to pick provider, converts format if needed
9. Injects real API key, forwards to upstream provider
10. Intercepts response, extracts token usage, calculates cost
11. Quality gate checks response (detects empty/truncated/refused)
12. Response policy applies redaction/truncation
13. Records request in SQLite/PostgreSQL
14. Caches response (if enabled)
15. Returns response with cost headers (`X-Cost-USD`, `X-Input-Tokens`, `X-Output-Tokens`)

### Request flow (with MCP tools)

1. Steps 1-6 same as above
2. Proxy injects tool definitions, forces `stream: false`
3. Sends to upstream LLM
4. If LLM returns `tool_calls`:
   - Proxy routes each call to appropriate MCP server
   - Executes tool, collects results
   - Appends assistant message + tool results
   - Loops back to step 3 (up to `max_iterations`)
5. When LLM returns without `tool_calls`:
   - Strips tool-related fields from response
   - Returns clean response to agent (never sees tools)
   - Records accumulated tokens/cost as single entry

### Session overrides

Sessions allow per-request config changes without modifying global config:

```bash
# Create session override
curl -X POST http://localhost:8080/v1/sessions/my-session \
  -H "Content-Type: application/json" \
  -d '{
    "agent_name": "my-agent",
    "model": "gpt-4o-mini",
    "temperature": 0.5,
    "max_tokens": 1000
  }'

# Use session in request
curl http://localhost:8080/v1/chat/completions \
  -H "X-Session-ID: my-session" \
  -H "X-Agent-Name: my-agent" \
  -d '{"model": "gpt-4o", "messages": [...]}'
  # Session override: uses gpt-4o-mini instead of gpt-4o
```

### Webhooks

Receive webhooks, render templates, execute LLM, fire callback:

```bash
# Define in config
webhooks:
  definitions:
    summarize:
      secret: "my_webhook_secret"
      model: "gpt-4o-mini"
      prompt_template: "Summarize this report:\n{{.Payload}}"
      callback_url: "https://api.example.com/callback"

# Send webhook (HMAC-SHA256 signed)
curl -X POST http://localhost:8080/v1/webhooks/summarize \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Signature: sha256=HMAC_HEX" \
  -d '{"title": "Sales Report", "data": "..."}'
```

## Development

```bash
cd tools/agix

# Build
go build -o agix .
make build

# Test
go test ./...
go test -v ./...
go test -run TestName ./internal/package/

# Lint
go vet ./...

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o agix .
GOOS=darwin GOARCH=arm64 go build -o agix .
```

## License

MIT
