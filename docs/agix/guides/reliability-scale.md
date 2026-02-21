# Reliability & Scale

## Overview

agix includes features for handling provider failures, controlling request rates, alerting on spending, and webhooks:

- **Multi-provider failover** — Automatic fallback chains on provider errors
- **Rate limiting** — Per-agent request throttling (requests/min and hour)
- **Budget alerts** — Webhook notifications when spending hits thresholds
- **Generic webhooks** — Receive webhooks, render templates, execute LLM, fire callbacks

## Multi-Provider Failover

Failover automatically routes requests to backup models when the primary fails.

### How It Works

1. Request arrives for model X
2. Proxy checks failover chain for X
3. Tries to call X
4. If X fails (API error, timeout, etc.):
   - Falls back to first alternative
   - Retries with new model
5. If alternative fails:
   - Tries next in chain
6. If all fail:
   - Returns error to agent

### Configuration

```yaml
failover:
  max_retries: 2                   # Retry up to 2 times

  chains:
    gpt-4o:
      - "gpt-4o-mini"              # Try mini if 4o fails
      - "gpt-35-turbo"             # Then try 3.5 turbo

    claude-opus-4-6:
      - "claude-sonnet-4-5-20250929"
      - "claude-haiku-4-5-20251001"

    deepseek-chat:
      - "gpt-4o"                   # Fall back to OpenAI
```

### Real-World Example: OpenAI Outage

Scenario: OpenAI API goes down

```
Request for gpt-4o arrives
  → OpenAI API returns 503
  → Fail, try fallback: gpt-4o-mini
  → OpenAI API returns 503 (still down)
  → Fail, try fallback: gpt-35-turbo
  → OpenAI API returns 503 (still down)
  → Return error to agent
```

Better setup with cross-provider failover:

```yaml
failover:
  chains:
    gpt-4o:
      - "gpt-4o-mini"              # Try cheaper OpenAI model
      - "claude-opus-4-6"          # Fall back to Anthropic
      - "deepseek-chat"            # Fall back to DeepSeek
```

Now:

```
Request for gpt-4o arrives
  → OpenAI API returns 503
  → Try gpt-4o-mini → also 503
  → Try claude-opus-4-6 → 200 OK ✓
  → Return response from Anthropic
```

### Cost Implications

Failover adds retries, increasing token usage:

```
Scenario: gpt-4o → gpt-4o-mini → claude-opus
- Request 1 (gpt-4o): 5,000 input tokens → fails
- Request 2 (gpt-4o-mini): 5,000 input tokens → fails
- Request 3 (claude-opus): 5,000 input tokens → succeeds
- Total: 15,000 input tokens (3x normal)
- Cost: 3x normal cost
```

**Recommendation**: Set `max_retries: 1` to limit retry cost.

## Rate Limiting

Rate limiting controls how many requests each agent can make.

### How It Works

1. Request arrives with `X-Agent-Name` header
2. Proxy checks rate limit for the agent
3. If agent under limit:
   - Request allowed through
   - Counter incremented
4. If agent at/over limit:
   - Request rejected with 429 status
   - Response includes `Retry-After` header

### Configuration

```yaml
rate_limits:
  expensive-agent:
    requests_per_minute: 10
    requests_per_hour: 100

  default-agent:
    requests_per_minute: 30
    requests_per_hour: 500

  # Agents not listed have no limit
```

### Response Headers

When rate limited:

```
HTTP/1.1 429 Too Many Requests
Retry-After: 6                   # Wait 6 seconds

{
  "error": {
    "message": "Rate limit exceeded for agent: expensive-agent (10 req/min)",
    "type": "rate_limit_exceeded"
  }
}
```

### Real-World Example: Preventing Runaway Agents

```yaml
# Agent is calling LLM in a tight loop
expensive-agent:
  requests_per_minute: 5           # Max 5 requests/min
  requests_per_hour: 100           # Max 100/hour

# Without rate limit:
# - Agent sends 1000 requests in 1 minute
# - Proxy processes all of them
# - Cost: $150 (if each request costs $0.15)
# - User discovers cost after 1 hour of running

# With rate limit:
# - Agent sends 1000 requests
# - Only 5 per minute are allowed
# - Others get 429 Retry-After: 12
# - Agent can implement backoff
# - Max cost per hour: $12.50
```

### Implementing Backoff in Agents

```python
import time
from openai import OpenAI, RateLimitError

client = OpenAI(base_url="http://localhost:8080/v1", api_key="unused")

max_retries = 3
for attempt in range(max_retries):
    try:
        response = client.chat.completions.create(
            model="gpt-4o",
            messages=[...],
            extra_headers={"X-Agent-Name": "expensive-agent"}
        )
        break
    except RateLimitError as e:
        if attempt < max_retries - 1:
            # Wait for retry_after seconds
            wait_seconds = int(e.response.headers.get("Retry-After", 60))
            print(f"Rate limited, waiting {wait_seconds} seconds...")
            time.sleep(wait_seconds)
        else:
            raise
```

## Budget Alerts

Budget alerts notify you via webhook when an agent's spending reaches certain thresholds.

### How It Works

1. Agent makes request
2. Proxy checks: current_spend vs daily_limit
3. If spend_percent ≥ alert_threshold:
   - Fire webhook with alert data
   - Include agent name, amount spent, limit

### Configuration

```yaml
budgets:
  code-reviewer:
    daily_limit_usd: 10.0
    monthly_limit_usd: 200.0
    alert_at_percent: 80         # Alert when 80% spent ($8.00)

# Webhook destination
webhooks:
  definitions:
    budget-alert:
      secret: "webhook_secret_key"
      model: "gpt-4o-mini"
      prompt_template: |
        Alert: Agent {{.AgentName}} has reached {{.SpentPercent}}% of daily budget.

        Current spend: ${{.Spent}}
        Daily limit: ${{.Limit}}
        Remaining: ${{.Remaining}}

        Time: {{.Timestamp}}

      callback_url: "https://slack.example.com/alerts"
```

### Webhook Signature

Webhooks are signed with HMAC-SHA256:

```
Header: X-Webhook-Signature: sha256=<hex>
Secret: Used to sign the payload
```

Verify in your webhook handler:

```python
import hmac
import hashlib

def verify_webhook(request, secret):
    signature = request.headers.get("X-Webhook-Signature", "")
    body = request.get_data()

    expected = "sha256=" + hmac.new(
        secret.encode(),
        body,
        hashlib.sha256
    ).hexdigest()

    return hmac.compare_digest(signature, expected)
```

## Generic Webhooks

Generic webhooks allow you to receive HTTP events, process them with an LLM, and fire a callback.

### How It Works

1. External system sends HTTP POST to webhook endpoint
2. Proxy receives webhook payload
3. Renders template with payload data
4. Sends prompt to LLM (specified model)
5. Gets LLM response
6. Posts callback with LLM output to callback_url

### Configuration

```yaml
webhooks:
  enabled: true

  definitions:
    summarize-report:
      secret: "webhook_secret"
      model: "gpt-4o-mini"
      prompt_template: |
        Summarize this report in 3 bullet points:

        {{.Payload}}

      callback_url: "https://api.example.com/callback"

    translate-content:
      secret: "webhook_secret"
      model: "gpt-4o"
      prompt_template: |
        Translate the following to Spanish:

        {{.Payload}}

      callback_url: "https://api.example.com/translated"
```

### Sending a Webhook

Payload file (`webhook-payload.json`):

```json
{
  "title": "Sales Report",
  "data": "Q1 revenue up 25%"
}
```

Send the webhook with HMAC signature:

```bash
SECRET="webhook_secret"
PAYLOAD=$(cat webhook-payload.json)
SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$SECRET" -hex | sed 's/^.* //')

curl -X POST http://localhost:8080/v1/webhooks/summarize-report \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Signature: sha256=$SIGNATURE" \
  -d "$PAYLOAD"
```

### Real-World Example: Content Processing Pipeline

```yaml
webhooks:
  definitions:
    process-user-feedback:
      secret: "feedback_secret"
      model: "gpt-4o-mini"
      prompt_template: |
        Classify this customer feedback as positive, negative, or neutral.
        Extract key topics.
        Suggest one-sentence response.

        Feedback:
        {{.Payload}}

        Respond as JSON with keys: sentiment, topics, suggested_response

      callback_url: "https://api.example.com/feedback/processed"

# User submits feedback via web form
# Web app sends webhook to agix
# agix processes with LLM
# Result sent to CRM system
```

### Template Variables

Available in `prompt_template`:

| Variable | Type | Example |
|----------|------|---------|
| `{{.Payload}}` | string | Raw JSON/text from webhook |
| `{{.Headers}}` | map | Request headers |
| `{{.Timestamp}}` | string | ISO 8601 timestamp |
| `{{.WebhookName}}` | string | Webhook definition name |

### Webhook History

```bash
# View recent webhook executions
agix webhook history

# Output
# Timestamp          Webhook            Status  LLM Model      Latency
# 2026-02-21 10:15   summarize-report   SUCCESS gpt-4o-mini    450ms
# 2026-02-21 10:16   process-feedback   SUCCESS gpt-4o         1200ms
# 2026-02-21 10:17   translate-content  ERROR   gpt-4o         (connection timeout)
```

## Combining Reliability Features

### Example: Enterprise Production Setup

```yaml
# 1. Cross-provider failover
failover:
  max_retries: 1                   # Limit retry cost
  chains:
    gpt-4o:
      - "gpt-4o-mini"              # Try cheaper model first
      - "claude-opus-4-6"          # Cross-provider fallback

    claude-opus-4-6:
      - "claude-sonnet-4-5-20250929"  # Try cheaper Anthropic
      - "gpt-4o"                   # Cross-provider fallback

# 2. Rate limiting per agent
rate_limits:
  critical-agent:
    requests_per_minute: 30
    requests_per_hour: 500

  batch-agent:
    requests_per_minute: 5
    requests_per_hour: 100

# 3. Budget enforcement + alerts
budgets:
  critical-agent:
    daily_limit_usd: 100.0
    alert_at_percent: 75

webhooks:
  definitions:
    budget-alert:
      secret: "secret_key"
      model: "gpt-4o-mini"
      prompt_template: |
        Alert: Agent {{.AgentName}} at {{.SpentPercent}}%
      callback_url: "https://slack.example.com/alerts"

# 4. Detailed observability
audit:
  enabled: true

tracing:
  enabled: true
  sample_rate: 0.1                 # Sample 10% of requests
```

## Best Practices

1. **Set up failover chains** — Always have a backup provider
2. **Use rate limiting defensively** — Catch runaway agents early
3. **Set budget alerts low** — Get warnings at 75-80%, not 95%
4. **Implement webhook backoff** — Handle temporary callback failures
5. **Monitor failover usage** — High failover rates indicate provider issues
6. **Test failover during low traffic** — Don't discover failures in production
7. **Keep webhook payloads small** — Large payloads slow down processing
8. **Verify webhook signatures** — Always validate HMAC before processing
