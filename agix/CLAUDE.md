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
│  Write record to SQLite                  │
├─────────────────────────────────────────┤
│  SQLite (WAL mode, single file)          │
│  ~/.agix/agix.db             │
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
│   └── tools.go                       # `agix tools list` - MCP tools
├── internal/
│   ├── config/
│   │   ├── config.go                  # YAML config read/write
│   │   └── config_test.go
│   ├── proxy/
│   │   ├── proxy.go                   # HTTP reverse proxy + tool loop
│   │   └── proxy_test.go
│   ├── store/
│   │   ├── sqlite.go                  # SQLite storage layer
│   │   └── sqlite_test.go
│   ├── pricing/
│   │   ├── models.go                  # Model pricing table
│   │   └── models_test.go
│   ├── mcp/
│   │   ├── client.go                  # MCP client (stdio JSON-RPC 2.0)
│   │   └── client_test.go
│   ├── toolmgr/
│   │   ├── manager.go                 # Tool manager (aggregate + filter + route)
│   │   └── manager_test.go
│   └── ui/
│       ├── color.go                   # Terminal color utilities
│       └── color_test.go
```

## Tech stack

| Component | Choice | Reason |
|-----------|--------|--------|
| Language | Go 1.26 | Single binary, cross-compile, great for network I/O |
| CLI | github.com/spf13/cobra | Industry standard (Docker, K8s, gh CLI) |
| Database | modernc.org/sqlite | Pure Go, zero CGO, zero external deps |
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
database: "/Users/you/.agix/agix.db"
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
agix init                          # Create config with defaults
agix start [--port 8080]           # Start proxy
agix stats                         # Overall stats (today)
agix stats --by agent              # Per-agent breakdown
agix stats --by model              # Per-model breakdown
agix stats --by day                # Daily costs
agix stats --period 2026-01        # Specific month
agix logs                          # Recent 50 requests
agix logs --tail                   # Live tail (poll 500ms)
agix logs --agent code-reviewer    # Filter by agent
agix logs -n 100                   # Last 100 requests
agix budget list                   # Show all budgets
agix budget set <agent> [flags]    # Set budget
agix budget remove <agent>         # Remove budget
agix export --format csv           # Export CSV
agix export --format json          # Export JSON
agix export --period 2026-01       # Specific month
agix tools list                    # List all MCP tools
```

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

## Roadmap (not yet implemented)

- Webhook alerts when budget thresholds are hit
- Web dashboard (optional, serve from the same binary)
- Prompt caching stats
- Multi-provider failover
- Rate limiting per agent
- Request/response content logging (opt-in, for debugging)
