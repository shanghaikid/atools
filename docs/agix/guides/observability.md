# Observability

## Overview

agix provides comprehensive observability to monitor performance, debug issues, and track security events:

- **Request tracing** — Detailed per-request spans showing all pipeline steps
- **Audit logging** — Security event log with optional content capture
- **Health checks** — `agix doctor` command to diagnose configuration and connectivity
- **Metrics dashboard** — Web UI with cost visualization and budget monitoring

## Request Tracing

Request tracing provides detailed timing and diagnostics for each request flowing through the proxy.

### How It Works

Every request receives a `Trace-ID` and generates multiple spans:

1. Request arrives → create root span
2. Each step in the pipeline creates a child span:
   - Budget check
   - Firewall scan
   - Prompt template injection
   - Cache lookup
   - Router analysis
   - Provider API call
   - Token counting
   - Cost calculation
   - Response policy
   - Database write
3. All spans collected under the trace ID
4. Trace available for inspection

### Viewing Traces

```bash
# List recent traces
agix trace list

# View detailed trace with all spans
agix trace abc-123

# Filter by agent
agix trace list --agent code-reviewer
```

Example output:

```
Trace ID: trace-abc-123
Status: SUCCESS
Duration: 1250ms
Request: gpt-4o, 250 input tokens

Spans:
├─ proxy.request (1250ms)
│  ├─ budget.check (5ms)
│  ├─ firewall.scan (15ms)
│  ├─ cache.lookup (20ms)
│  ├─ router.classify (10ms)
│  ├─ api.call (1000ms)
│  │  └─ openai.chat.completions
│  ├─ extraction.tokencount (15ms)
│  ├─ cost.calculate (5ms)
│  ├─ policy.apply (10ms)
│  └─ store.write (50ms)

Cost: $0.015
```

### Response Header

Every response includes the trace ID:

```
X-Trace-ID: trace-abc-123
```

Save this to correlate with backend logs.

### Tracing Configuration

```yaml
tracing:
  enabled: true
  sample_rate: 1.0                 # 0-1, log all by default

  # sample_rate examples:
  # 1.0 = log all requests (verbose, high disk usage)
  # 0.5 = log 50% of requests (balanced)
  # 0.1 = log 10% of requests (performance, sampling)
  # 0.0 = disable tracing
```

### Use Case: Debug Slow Requests

```bash
# Identify slow traces
agix trace list | grep "Duration: [2-9][0-9][0-9][0-9]ms"

# View detailed span breakdown
agix trace trace-slow-123

# Check which span is bottleneck:
# If api.call > 1000ms → upstream provider is slow
# If cache.lookup > 100ms → database issue
# If firewall.scan > 100ms → regex performance issue
```

## Audit Logging

Audit logging tracks security events: tool calls, firewall blocks, policy applications, authentication.

### How It Works

1. Security-relevant event occurs (tool call, block, redaction)
2. Event logged with:
   - Timestamp
   - Agent name
   - Event type
   - Details (tool name, block reason, etc.)
   - Optional content (request/response if enabled)
3. Log persists to database

### Configuration

```yaml
audit:
  enabled: true
  content_log: true                # Log full request/response bodies

  # Flag high-risk tools as dangerous
  dangerous_tools: ["delete_file", "execute_cmd", "modify_permissions"]
```

### Viewing Audit Events

```bash
# List security events
agix audit list

# Filter by event type
agix audit list --type tool_call
agix audit list --type firewall_block
agix audit list --type response_redaction

# Filter by agent
agix audit list --agent code-reviewer

# Show more events
agix audit list -n 50
```

Example output:

```
Timestamp          Agent           Type               Details
2026-02-21 10:15   code-reviewer   tool_call          called: read_file (args: src/main.go)
2026-02-21 10:16   docs-writer     tool_call          called: write_file (args: README.md)
2026-02-21 10:17   security-scan   firewall_block     rule: block_injection matched
2026-02-21 10:18   test-agent      response_redaction emails masked: 3 occurrences
```

### Event Types

| Type | Meaning | Risk |
|------|---------|------|
| `tool_call` | Tool was executed | Medium (tools can modify system) |
| `firewall_block` | Injection attempt blocked | High |
| `firewall_warn` | Suspicious pattern detected | Medium |
| `response_redaction` | Output was redacted | Low |
| `budget_exceeded` | Agent hit budget limit | Low |
| `rate_limit_exceeded` | Agent exceeded rate limit | Low |

### Content Logging

When `content_log: true`, request/response bodies are captured:

```bash
agix audit list --type tool_call -n 1 | jq '.content'

# Output:
# {
#   "request": {
#     "model": "gpt-4o",
#     "messages": [...]
#   },
#   "response": {
#     "content": "..."
#   }
# }
```

**⚠️ Security note**: Content logs may contain sensitive data. Keep them secure and set retention policies.

### Real-World Example: Investigating Tool Misuse

```bash
# 1. Agent started calling delete_file unexpectedly
# 2. Review tool call history
agix audit list --type tool_call --agent suspicious-agent -n 100

# 3. Look for the delete_file call
# Output shows: agent called delete_file with sensitive paths

# 4. Revoke the agent's tool access
vim ~/.agix/config.yaml
# Edit: suspicious-agent: deny: ["delete_file"]

# 5. Restart proxy
agix start
```

## Health Checks (agix doctor)

The `agix doctor` command performs comprehensive diagnostics to verify configuration and connectivity.

### Running Doctor

```bash
agix doctor
```

Output example:

```
Configuration Health Check
  ✓ Config file exists at ~/.agix/config.yaml
  ✓ Config file permissions OK (0600)
  ✓ All required fields present

API Key Validation
  ✓ OpenAI API key valid
  ✓ Anthropic API key valid
  ✓ DeepSeek API key valid

Database
  ✓ SQLite database accessible
  ✓ Database integrity check passed
  ✓ Schema version: 1

MCP Servers
  ✓ filesystem server started (pid: 12345)
  ✓ github server started (pid: 12346)

Budget Configuration
  ✓ All budgets logically consistent

Firewall Rules
  ✓ All regex patterns compile successfully

Overall Status: PASS (all checks passed)
```

### Doctor Checks

Doctor performs these checks:

1. **Config file permissions** — Verify 0600 (owner only)
2. **API key validity** — Test actual API calls to each provider
3. **Database connectivity** — Check file exists and is healthy
4. **MCP server startup** — Verify tools can be executed
5. **Budget logic** — Daily ≤ monthly, percentage in range
6. **Firewall rules** — Validate regex patterns
7. **Prompt template validity** — Check template syntax

### Common Doctor Issues

**Issue**: "API key invalid"

```
Cause: Key expired or wrong format
Fix: Update key in config.yaml
$ vim ~/.agix/config.yaml
```

**Issue**: "Database integrity check failed"

```
Cause: Corrupted SQLite file
Fix: Backup and rebuild
$ cp ~/.agix/agix.db ~/.agix/agix.db.backup
$ rm ~/.agix/agix.db
# Next agix command will recreate it
```

**Issue**: "MCP server failed to start"

```
Cause: Command not found or syntax error
Fix: Verify command in config
$ npm list -g @modelcontextprotocol/server-filesystem
# Install if missing
$ npm install -g @modelcontextprotocol/server-filesystem
```

## Metrics Dashboard

The web dashboard provides real-time visualization of costs and budgets.

### Enabling Dashboard

```yaml
dashboard:
  enabled: true                    # Serves at /dashboard/
```

### Accessing Dashboard

```
http://localhost:8080/dashboard/
```

### Dashboard Features

1. **Cost Overview**
   - Today's total cost
   - Daily trend (last 30 days)
   - Cost by agent (pie chart)
   - Cost by model (bar chart)

2. **Budget Monitoring**
   - Budget status per agent
   - Visual progress bars (0-100%)
   - Remaining budget in USD
   - Days until reset

3. **Real-Time Metrics**
   - Current request rate
   - Average response time
   - Cache hit rate
   - Token efficiency

4. **Alerts**
   - Agents approaching budget limit
   - Recent firewall blocks
   - Failed requests

### API Endpoints (for custom dashboards)

```bash
# Get aggregated stats
curl http://localhost:8080/api/stats

# Get per-agent stats
curl http://localhost:8080/api/agents

# Get budget info
curl http://localhost:8080/api/budgets

# Get daily costs
curl http://localhost:8080/api/costs/daily

# Get recent logs
curl http://localhost:8080/api/logs
```

Example JSON response:

```json
{
  "stats": {
    "total_requests": 1245,
    "total_cost_usd": 18.50,
    "total_input_tokens": 125000,
    "total_output_tokens": 45000,
    "avg_cost_per_request": 0.015,
    "cache_hit_rate": 0.23
  },
  "by_agent": [
    {
      "agent": "code-reviewer",
      "requests": 450,
      "cost": 12.30,
      "cache_hits": 120
    },
    {
      "agent": "docs-writer",
      "requests": 300,
      "cost": 4.20,
      "cache_hits": 45
    }
  ]
}
```

## Observability Pipeline

### Real-World Monitoring Setup

```yaml
# 1. Enable all observability features
tracing:
  enabled: true
  sample_rate: 0.5              # Log 50% of requests

audit:
  enabled: true
  content_log: false            # Don't log full bodies (privacy)

dashboard:
  enabled: true

# 2. Set up periodic reviews
# Morning:
#   agix stats --period 1d      # Yesterday's costs
#   agix trace list             # Check for slow requests
#
# Weekly:
#   agix audit list -n 1000     # Review security events
#   agix stats --group-by agent # Agent cost breakdown
#
# Monthly:
#   agix export --format csv --period 2026-02  # Export for reporting
#   agix doctor                 # Verify system health
```

### Alerting Pattern

```bash
#!/bin/bash
# Run daily via cron

ALERT_EMAIL="ops@example.com"

# Check if any agent exceeded 80% of daily budget
HIGH_SPEND=$(agix budget | grep "%" | grep -E "[8-9][0-9]%|100%")

if [ ! -z "$HIGH_SPEND" ]; then
  echo "⚠️  High spending detected:" > /tmp/alert.txt
  echo "$HIGH_SPEND" >> /tmp/alert.txt
  mail -s "agix: Budget Alert" $ALERT_EMAIL < /tmp/alert.txt
fi

# Check for firewall blocks
BLOCKS=$(agix audit list --type firewall_block -n 10)
if [ ! -z "$BLOCKS" ]; then
  echo "🚨 Firewall blocks detected:" > /tmp/alert.txt
  echo "$BLOCKS" >> /tmp/alert.txt
  mail -s "agix: Firewall Alert" $ALERT_EMAIL < /tmp/alert.txt
fi
```

## Best Practices

1. **Enable tracing for debugging** — Helps identify performance bottlenecks
2. **Monitor firewall blocks** — Daily review of audit logs for suspicious activity
3. **Run doctor before deployment** — Verify all dependencies and configs
4. **Check dashboard daily** — Catch runaway spending early
5. **Set up alerting** — Automated notifications for budget/security events
6. **Sample traces in production** — Use `sample_rate: 0.1-0.5` to reduce disk usage
7. **Retain audit logs** — Keep for 90+ days for compliance
8. **Export monthly data** — CSV exports for finance/reporting
