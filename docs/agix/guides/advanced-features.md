# Advanced Features

## Overview

agix includes several advanced capabilities for specialized use cases:

- **System prompt injection** — Inject global or per-agent system prompts
- **MCP tool bundles** — Pre-packaged tool sets for common workflows
- **PostgreSQL backend** — Scalable alternative to SQLite for large deployments
- **DeepSeek provider support** — Route to DeepSeek models alongside OpenAI/Anthropic

## System Prompt Injection

System prompt injection allows you to prepend or append text to the system prompt of every request.

### How It Works

1. Request arrives from agent
2. If prompt injection enabled:
   - Select applicable prompt (global or per-agent)
   - Prepend or append to existing system prompt
3. Forward modified request to LLM
4. LLM processes with injected prompt

### Configuration

```yaml
prompt_templates:
  enabled: true

  # Global template applied to all agents
  global: "You are a helpful assistant. Follow company policies."

  # Per-agent overrides
  agents:
    code-reviewer: "You are an expert code reviewer. Focus on security and performance."
    docs-writer: "You are a technical writer. Use clear, concise language."
    compliance-checker: "You are a compliance officer. Check all responses against policy."

  # Position: prepend (before user prompt) or append (after)
  position: "prepend"              # Default: prepend
```

### Real-World Example: Enforcing Company Policies

```yaml
prompt_templates:
  enabled: true

  global: |
    You are a company AI assistant.

    IMPORTANT:
    - Never recommend competitor products
    - Always mention company products first
    - Adhere to data privacy regulations
    - Do not engage in political discussions

  agents:
    customer-support:
      template: |
        You are a friendly customer support agent.
        Prioritize customer satisfaction.
        Offer the best solution, not the most profitable.
      position: "prepend"

    sales-agent:
      template: |
        You are a sales assistant.
        Recommend company products that fit customer needs.
        Highlight competitive advantages.
```

### Impact on Cost

Injected prompts increase token usage:

```
Without injection:
- User: "Hello" (10 tokens)
- LLM processes: 10 tokens

With injection:
- System: "You are a helpful assistant..." (20 tokens)
- User: "Hello" (10 tokens)
- LLM processes: 30 tokens
- Cost increase: 2x tokens for system prompt
```

### Position: Prepend vs Append

**Prepend** (default):
- System prompt comes first
- Has priority in LLM reasoning
- More reliable

**Append**:
- System prompt comes after user message
- User message has priority
- Useful for soft guidelines

```yaml
# Example: Soft guideline (append)
agents:
  flexible-agent:
    template: "Try to be helpful and not boring."
    position: "append"
```

## MCP Tool Bundles

Tool bundles are pre-packaged sets of MCP servers for common workflows.

### Built-In Bundles

agix includes several pre-configured bundles:

```bash
# List available bundles
agix bundle list

# Output:
# Name          Description
# basic         File operations (read, write, browse)
# github        GitHub repo access (read, search, contribute)
# code-review   Code analysis and review tools
# devops        Infrastructure and deployment tools
# docs-writer   Documentation and publishing tools
```

### Installing a Bundle

```bash
# Install a bundle
agix bundle install basic

# Show bundle details
agix bundle show github
# Output:
# Bundle: github
# Description: GitHub repository access
# Servers:
#   - github: GitHub client (requires GITHUB_TOKEN env var)
```

### Creating Custom Bundles

Define a bundle in config:

```yaml
bundles: ["basic", "github", "custom"]

tools:
  servers:
    # Bundle: basic (implicit, from built-in)

    # Bundle: github (implicit, from built-in)

    # Custom tools (not in bundles)
    custom-api:
      command: "npx"
      args: ["-y", "@company/custom-api-server"]
      env: ["API_KEY=..."]

    internal-tools:
      command: "/usr/local/bin/internal-tools-server"
```

Then reference in config:

```yaml
bundles:
  - basic           # Use built-in bundle
  - github          # Use built-in bundle
  - code-review     # Would use built-in if available
```

### Tool Access Control

Control which agents can use which tools:

```yaml
tools:
  servers:
    filesystem:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]

    github:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-github"]
      env: ["GITHUB_TOKEN=ghp_xxx"]

  agents:
    # Code reviewer: read-only file access
    code-reviewer:
      allow: ["read_file", "list_directory"]

    # Docs writer: full filesystem + github
    docs-writer:
      allow: ["*"]                 # All tools allowed

    # Security agent: only sandbox execution
    security-agent:
      deny: ["delete_file", "modify_permissions"]

    # Everyone else: no tools by default
```

### Real-World Example: Documentation Workflow

```bash
# Install documentation bundle
agix bundle install docs-writer

# Config:
bundles:
  - docs-writer              # Includes filesystem + github + publishing tools

tools:
  agents:
    docs-agent:
      allow: ["read_file", "write_file", "list_directory", "git_commit"]
```

Now `docs-agent` can:
- Read code files for examples
- Write documentation
- Commit changes to git

All without agent code changes!

## PostgreSQL Backend

For large deployments with many requests, PostgreSQL provides better scalability than SQLite.

### Setup

1. Create PostgreSQL database:

```bash
createdb agix
psql agix -c "CREATE ROLE agix WITH PASSWORD 'password' CREATEDB;"
ALTER ROLE agix CREATEDB;
```

2. Configure agix:

```yaml
database: "postgres://agix:password@localhost:5432/agix?sslmode=disable"
```

3. Run agix (schema created automatically):

```bash
agix start
```

### Configuration Variations

**Production (with SSL):**
```yaml
database: "postgres://agix:password@prod-db.example.com:5432/agix?sslmode=require"
```

**With SSL certificate:**
```yaml
database: "postgres://agix:password@prod-db.example.com:5432/agix?sslmode=verify-full&sslrootcert=/path/to/ca.pem"
```

**Connection pooling via pgBouncer:**
```yaml
database: "postgres://agix:password@pgbouncer.example.com:6432/agix?sslmode=disable"
```

### SQLite vs PostgreSQL

| Feature | SQLite | PostgreSQL |
|---------|--------|------------|
| Setup | Automatic (file) | Requires server |
| Queries | Single-machine | Distributed |
| Concurrency | Good (WAL mode) | Excellent |
| Disk space | MB-GB | Unlimited |
| Backups | File copy | `pg_dump` |
| Replication | None | Yes (streaming) |

**Use SQLite when:**
- Single server
- <1M requests/day
- Don't need distributed backup

**Use PostgreSQL when:**
- Multiple servers
- >1M requests/day
- Need high availability
- Require backup/replication

### Migration: SQLite → PostgreSQL

```bash
# 1. Export SQLite data
agix export --format json > backup.json

# 2. Update config to use PostgreSQL
vim ~/.agix/config.yaml
# database: "postgres://..."

# 3. Restart agix (schema created)
agix start

# 4. Import data (manual, depends on your script)
# Use the JSON backup to populate PostgreSQL
```

### Performance Tuning

For PostgreSQL with high request volume:

```sql
-- Create indexes for common queries
CREATE INDEX idx_requests_agent_timestamp
  ON requests (agent_name, timestamp DESC);

CREATE INDEX idx_requests_model
  ON requests (model, timestamp DESC);

-- Analyze for query optimization
ANALYZE requests;
```

## DeepSeek Provider Support

agix supports DeepSeek models alongside OpenAI and Anthropic.

### Configuration

```yaml
keys:
  openai: "sk-..."
  anthropic: "sk-ant-..."
  deepseek: "sk-..."

# DeepSeek models are routed automatically
# Just use model names like "deepseek-chat"
```

### Supported Models

```
deepseek-chat        # General purpose chat
deepseek-reasoner    # Reasoning/analysis
```

### Routing

Model name → Provider mapping:

```
gpt-* → OpenAI
claude-* → Anthropic
deepseek-* → DeepSeek
```

### Cost Example

Comparing providers:

```
Task: Summarize 2000 words

OpenAI (gpt-4o-mini):
- Input: 500 tokens × $0.00015 = $0.075
- Output: 200 tokens × $0.0006 = $0.00012
- Total: $0.07512

Anthropic (claude-haiku):
- Input: 500 tokens × $0.00008 = $0.04
- Output: 200 tokens × $0.00024 = $0.000048
- Total: $0.040048

DeepSeek (deepseek-chat):
- Input: 500 tokens × $0.00014 = $0.07
- Output: 200 tokens × $0.00028 = $0.000056
- Total: $0.070056

Best option: Anthropic Haiku (55% cheaper than GPT-4o-mini)
```

### Failover Pattern: Cross-Provider

```yaml
failover:
  chains:
    gpt-4o:
      - "gpt-4o-mini"         # Cheaper OpenAI
      - "deepseek-chat"       # Cross-provider
      - "claude-opus-4-6"     # Last resort

    claude-opus-4-6:
      - "claude-sonnet-4-5-20250929"  # Cheaper Anthropic
      - "deepseek-chat"              # Cross-provider
      - "gpt-4o"                     # Last resort
```

## Combining Advanced Features

### Real-World Example: Enterprise SaaS Setup

```yaml
# PostgreSQL for scalability
database: "postgres://agix:password@prod-db.example.com:5432/agix?sslmode=require"

# System prompts for safety
prompt_templates:
  enabled: true
  global: |
    You are an enterprise AI assistant.
    Follow GDPR and company security policies.
  agents:
    customer-support:
      template: "Always be helpful and professional."

# Tool bundles for productivity
bundles:
  - basic
  - github
  - code-review

tools:
  agents:
    developer-bot:
      allow: ["*"]                # Full access for trusted bot

    customer-support:
      deny: ["delete_file", "modify_permissions", "git_commit"]  # Read-only

# Cross-provider failover
failover:
  chains:
    gpt-4o:
      - "gpt-4o-mini"
      - "claude-opus-4-6"
      - "deepseek-chat"

# Rate limiting + budgets
rate_limits:
  api-consumer:
    requests_per_minute: 100
    requests_per_hour: 5000

budgets:
  api-consumer:
    daily_limit_usd: 50.0
    monthly_limit_usd: 1000.0
    alert_at_percent: 80
```

## Best Practices

1. **Use system prompts sparingly** — Only for critical policies, not general guidance
2. **Install only needed bundles** — Reduces complexity and tool surface area
3. **Migrate to PostgreSQL early** — Better to plan this before high traffic
4. **Cross-provider failover** — DeepSeek as cheap fallback, Anthropic for quality
5. **Test provider switching** — Quality may differ between providers
6. **Monitor bundle updates** — Built-in bundles may get new/changed tools
