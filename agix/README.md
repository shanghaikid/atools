# agix

LLM reverse proxy for AI agents. Tracks every token, calculates cost, enforces per-agent budgets, and optionally injects shared MCP tools — all transparently, with zero agent-side changes.

```
Agent → agix (localhost:8080) → OpenAI / Anthropic
              ↓
         SQLite (usage log)
              ↓
      CLI: stats · logs · budget · export
```

## Features

- **Cost tracking** — records tokens, calculates cost for every request, stores in local SQLite
- **Budget enforcement** — per-agent daily/monthly limits, returns 429 when exceeded
- **Shared MCP tools** — inject tools from MCP servers into LLM conversations, agents don't need any code changes
- **Multi-provider** — routes `gpt-*` to OpenAI, `claude-*` to Anthropic, auto-converts between API formats
- **API key isolation** — agents never see real API keys, proxy injects them
- **Streaming support** — transparent SSE pass-through with usage extraction
- **Single binary, zero external deps** — pure Go, no CGO, cross-compiles to any platform

## Quick start

```bash
# Build
make build

# Initialize config
./agix init

# Add your API keys
vim ~/.agix/config.yaml

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
# Build from source
make build

# Install to /usr/local/bin
make install

# Or cross-compile
GOOS=linux GOARCH=amd64 go build -o agix .
```

## Configuration

Config lives at `~/.agix/config.yaml` (created by `agix init`):

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
  max_iterations: 10
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
      deny: ["write_file", "delete_file"]       # blacklist
    # agents not listed get all tools
```

## CLI

```bash
agix init                             # Create config with defaults
agix start [--port 8080]              # Start proxy

agix stats                            # Today's stats
agix stats --period 7d                # Last 7 days
agix stats --group-by agent           # Per-agent breakdown
agix stats --group-by model           # Per-model breakdown
agix stats --group-by day             # Daily costs

agix logs                             # Last 20 requests
agix logs --tail                      # Watch in real-time
agix logs --agent my-agent            # Filter by agent
agix logs -n 100                      # Last 100 requests

agix budget                           # Show all budgets + current spend
agix budget set -a my-agent -d 5.00   # Set daily limit
agix budget set -a my-agent -m 100    # Set monthly limit
agix budget remove -a my-agent        # Remove budget

agix export                           # CSV to stdout
agix export --format json             # JSON to stdout
agix export -o costs.csv              # CSV to file
agix export --period 30d              # Specific period

agix tools list                       # List discovered MCP tools
```

## How it works

### Without tools

1. Agent sends `POST /v1/chat/completions` (OpenAI-compatible format)
2. Proxy reads `model` to pick provider, converts format if needed
3. Injects real API key, forwards to upstream
4. Intercepts response, extracts token usage, calculates cost
5. Writes record to SQLite, returns response to agent
6. Adds `X-Cost-USD`, `X-Input-Tokens`, `X-Output-Tokens` response headers

### With MCP tools

1. Agent sends a normal request (unaware of tools)
2. Proxy injects tool definitions, forces non-streaming
3. If LLM returns `tool_calls`: proxy executes via MCP server, appends results, loops back
4. When LLM returns without `tool_calls`: strips tool fields, returns clean response
5. Agent never sees tool-related messages — it just gets a final answer

### Endpoints

| Endpoint | Description |
|----------|-------------|
| `POST /v1/chat/completions` | Proxied chat completions |
| `GET /v1/models` | List available models |
| `GET /health` | Health check |

### Agent identification

Agents identify via the `X-Agent-Name` header. This enables per-agent stats, budget enforcement, and tool access control.

## Supported models

**OpenAI**: gpt-5.2, gpt-5.1, gpt-5, gpt-5-mini, gpt-5-nano, gpt-4.1, gpt-4.1-mini, gpt-4.1-nano, gpt-4o, gpt-4o-mini, o1, o3, o3-mini, o4-mini

**Anthropic**: claude-opus-4-6, claude-sonnet-4-5, claude-haiku-4-5, claude-3-5-haiku, claude-3-haiku

Versioned model names (e.g. `gpt-4o-2024-08-06`) are matched via longest-prefix against the pricing table.

## Development

```bash
go test ./...       # Run tests
go vet ./...        # Lint
make build          # Build binary
make clean          # Remove binary
```

## License

MIT
