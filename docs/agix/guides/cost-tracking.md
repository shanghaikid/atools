# Cost Tracking & Budget Management

## Overview

agix automatically tracks every LLM request, extracts token usage, and calculates costs based on provider pricing. This enables per-agent budget enforcement and detailed cost analytics.

## How Cost Tracking Works

### Request Flow

When an agent sends a request through agix:

1. Agent sends `POST /v1/chat/completions` with optional `X-Agent-Name` header
2. Proxy forwards to the upstream LLM provider
3. Response arrives with token counts (`prompt_tokens`, `completion_tokens`)
4. Proxy calculates cost using the pricing table:
   - Cost = (prompt_tokens × prompt_price) + (completion_tokens × completion_price)
5. Record is written to SQLite/PostgreSQL
6. Response headers include cost information

### Response Headers

Every response includes these headers:

```
X-Cost-USD: 0.015          # Cost in USD
X-Input-Tokens: 120        # Prompt tokens
X-Output-Tokens: 45        # Completion tokens
X-Trace-ID: trace-abc123   # For observability
```

### Data Storage

All requests are persisted to the database:

```sql
CREATE TABLE requests (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp     DATETIME NOT NULL,
    agent_name    TEXT NOT NULL,
    model         TEXT NOT NULL,
    provider      TEXT NOT NULL,
    input_tokens  INTEGER NOT NULL,
    output_tokens INTEGER NOT NULL,
    cost_usd      REAL NOT NULL,
    duration_ms   INTEGER NOT NULL,
    status_code   INTEGER NOT NULL
);
```

## Viewing Costs

### Overall Statistics

```bash
# Today's costs
agix stats

# Last 7 days
agix stats --period 7d

# Specific month (YYYY-MM format)
agix stats --period 2026-01

# JSON output
agix stats --format json
```

Example output:
```
Total requests:  245
Total input:     18,450 tokens
Total output:    8,920 tokens
Total cost:      $12.45
Avg cost/request: $0.051
```

### Per-Agent Breakdown

```bash
# Cost by agent
agix stats --group-by agent

# Cost by model
agix stats --group-by model

# Daily costs (great for charts)
agix stats --group-by day
```

### Request Logs

```bash
# Last 20 requests
agix logs

# Last 100 requests
agix logs -n 100

# Real-time tail
agix logs --tail

# Filter by agent
agix logs --agent code-reviewer
```

Log columns:
- Timestamp
- Agent name
- Model used
- Input/output tokens
- Cost (USD)
- Response time (ms)

## Budget Enforcement

### Setting Budgets

Configure budgets in `~/.agix/config.yaml`:

```yaml
budgets:
  code-reviewer:
    daily_limit_usd: 10.0
    monthly_limit_usd: 200.0
    alert_at_percent: 80      # Warn at 80% spent

  docs-writer:
    daily_limit_usd: 5.0
    monthly_limit_usd: 100.0
    alert_at_percent: 75
```

Or use CLI:

```bash
# Set both limits
agix budget set code-reviewer -d 10.0 -m 200.0

# Set only daily limit
agix budget set code-reviewer -d 5.0

# View budgets
agix budget

# Remove budget
agix budget remove code-reviewer
```

### Budget Behavior

When an agent hits their budget:

1. Request arrives at proxy
2. Proxy checks: current_spend vs daily_limit + monthly_limit
3. If over budget: returns `429 Too Many Requests`
4. Response includes `Retry-After` header with seconds to wait

Example error response:

```json
HTTP/1.1 429 Too Many Requests
Retry-After: 3600

{
  "error": {
    "message": "Daily budget exceeded for agent: code-reviewer ($10.00 limit, $10.15 spent)",
    "type": "budget_exceeded"
  }
}
```

### Fail-Open Safety

If the database is unavailable during budget check:

- Request is **allowed through** (fail-open)
- Cost is recorded when database recovers
- Budget check tries again on next request

This ensures temporary database issues don't block agents.

## Cost Optimization Patterns

### Pattern 1: Monitor High-Cost Agents

```bash
# Find expensive agents
agix stats --group-by agent

# Drill down on specific agent
agix logs --agent expensive-agent -n 50
agix stats --agent expensive-agent --period 7d
```

**Action**: Set a daily budget to control costs:

```bash
agix budget set expensive-agent -d 50.0
```

### Pattern 2: Track Model Costs

```bash
# Which models are most expensive?
agix stats --group-by model

# Track gpt-4o vs gpt-4o-mini usage
agix logs --model gpt-4o
agix logs --model gpt-4o-mini
```

**Action**: Enable smart routing to automatically use cheaper models for simple requests (see Smart Routing guide).

### Pattern 3: Daily Cost Trending

```bash
# Export daily costs
agix stats --group-by day --format json > daily_costs.json

# Visualize costs over time
agix stats --group-by day
```

### Pattern 4: Budget Alerts

Configure webhook notifications when spending hits thresholds:

```yaml
budgets:
  code-reviewer:
    daily_limit_usd: 10.0
    alert_at_percent: 80  # Alert at $8.00

webhooks:
  definitions:
    budget-alert:
      secret: "webhook_secret"
      model: "gpt-4o-mini"
      prompt_template: |
        Agent {{.AgentName}} has reached {{.SpentPercent}}% of daily budget.
        Spent: ${{.Spent}} / ${{.Limit}}
      callback_url: "https://api.example.com/alerts/budget"
```

## Pricing Table

agix includes pricing for these models:

### OpenAI
- gpt-5.2, gpt-5.1, gpt-5
- gpt-5-mini, gpt-5-nano
- gpt-4.1, gpt-4.1-mini, gpt-4.1-nano
- gpt-4o, gpt-4o-mini
- o1, o3, o3-mini, o4-mini

### Anthropic
- claude-opus-4-6
- claude-sonnet-4-5-20250929
- claude-haiku-4-5-20251001
- claude-3-5-haiku-20241022

### DeepSeek
- deepseek-chat
- deepseek-reasoner

**Note**: Versioned models (e.g., `gpt-4o-2024-08-06`) are matched via longest-prefix matching against the pricing table.

## Common Issues

### Q: Why is my cost calculation different from the LLM provider's invoice?

**A**: agix uses moment-of-use pricing from the model listings. Provider invoices may include:
- Volume discounts
- Batch processing rates
- Different pricing changes that occurred during the period
- Rounding differences

Use `agix stats --format json` to export raw data and compare with your provider invoice.

### Q: Can I set budgets in tokens instead of dollars?

**A**: Currently budgets are USD only. If you need token-based limits, use rate limiting:

```yaml
rate_limits:
  expensive-agent:
    requests_per_hour: 10  # Limit to 10 requests/hour
```

### Q: How do I export costs to a spreadsheet?

**A**: Use CSV export:

```bash
agix export --format csv -o costs.csv
agix export --format csv --period 2026-01 -o january_costs.csv
```

Open in Excel/Google Sheets for analysis.

### Q: Can budgets reset at a specific time?

**A**: Budgets reset at midnight UTC. If you need a different timezone:

1. Create multiple agents in your application with different UTC offset names
2. Or coordinate resets manually via API (set session overrides with TTL)

## Best Practices

1. **Set per-agent budgets** — prevents runaway costs from any single agent
2. **Monitor daily trends** — `agix stats --group-by day` weekly
3. **Use rate limiting** — complement budgets with per-agent rate limits
4. **Enable audit logging** — track which tools are called and when
5. **Configure alerts** — webhook notifications at 80% of budget
6. **Export regularly** — keep CSV exports for finance/reporting
