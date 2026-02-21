# Intelligence & Optimization

## Overview

agix includes several advanced features to optimize LLM request handling, reduce costs, and improve performance:

- **Smart routing** — Automatically use cheaper models for simple requests
- **Semantic caching** — Cache responses for similar prompts to avoid redundant LLM calls
- **Context compression** — Summarize old messages when conversations get long
- **A/B testing** — Run traffic-split experiments to compare model performance

## Smart Routing

Smart routing automatically routes simple requests to cheaper models while keeping complex requests on premium models.

### How It Works

1. Request arrives from agent
2. Proxy analyzes the request:
   - Number of messages in conversation
   - Total input tokens
   - Message complexity heuristics
3. Classifies as "simple" or "complex"
4. Routes to appropriate model based on classification

### Configuration

Enable and configure routing:

```yaml
routing:
  enabled: true

  tiers:
    simple:
      max_message_tokens: 500      # Messages under 500 tokens
      max_messages: 3              # Conversations with ≤3 messages

  model_map:
    gpt-4o:
      simple: "gpt-4o-mini"        # Route simple → mini
      complex: "gpt-4o"            # Keep complex on full model

    claude-opus-4-6:
      simple: "claude-haiku-4-5-20251001"
      complex: "claude-opus-4-6"
```

### Cost Impact Example

Scenario: Code review agent handling 100 daily requests

- **Without routing**: 100 × gpt-4o ($0.003 per request) = $0.30/day
- **With routing**:
  - 60 simple → gpt-4o-mini ($0.00015 per request) = $0.009
  - 40 complex → gpt-4o = $0.12
  - **Total: $0.129/day (57% savings)**

### Pattern: Monitoring Route Decisions

Unfortunately agix doesn't expose routing decisions in logs yet, but you can infer them:

1. Filter logs by model
2. Notice patterns: simple requests → mini, complex → full
3. Adjust tier thresholds if needed

## Semantic Caching

Semantic caching uses embeddings to find similar cached responses instead of re-querying the LLM.

### How It Works

1. Request arrives with prompt
2. Proxy generates embedding for the prompt
3. Searches cache for similar embeddings (cosine similarity > threshold)
4. If match found: return cached response (saves $$ and latency)
5. If no match: forward to LLM, cache the result

### Configuration

```yaml
cache:
  enabled: true
  similarity_threshold: 0.95       # 0-1, how similar (1=exact)
  ttl_minutes: 60                  # Cache expires after 60 min
```

### When to Use

**Good candidates for caching:**
- FAQs or common questions
- Documentation lookup agents
- Template-based responses
- Repetitive summarization tasks

**Poor candidates:**
- Highly dynamic content (news, current info)
- Personalized responses (different per user)
- Creative/generative tasks
- Real-time analysis

### Response Header

Cache hits are indicated in response headers:

```
X-Cache: HIT        # Response from cache
X-Cache: MISS       # Fresh LLM response
```

### Cache Hit Example

```bash
# First request (cache miss)
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "X-Agent-Name: doc-agent" \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "Explain OAuth"}]}'
# Response: X-Cache: MISS, X-Cost-USD: 0.15

# Second similar request (cache hit)
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "X-Agent-Name: doc-agent" \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "What is OAuth?"}]}'
# Response: X-Cache: HIT, X-Cost-USD: 0.00
```

### Tuning Similarity Threshold

- **0.99-1.0**: Only exact matches cached (low hit rate, safe)
- **0.95-0.99**: High similarity required (good default, minimal hallucination risk)
- **0.90-0.95**: More aggressive caching (monitor for wrong answers)
- **<0.90**: Not recommended (may return irrelevant cached responses)

## Context Compression

For long-running conversations, context windows fill up. Context compression automatically summarizes old messages to free up space.

### How It Works

1. Request arrives with N messages in conversation
2. Proxy checks: total_tokens > compression_threshold?
3. If yes:
   - Selects oldest messages (keeping recent ones)
   - Summarizes them using a lightweight model
   - Replaces original messages with summary
4. Forwards compressed conversation to LLM

### Configuration

```yaml
compression:
  enabled: true
  threshold_tokens: 50000          # Trigger when conversation exceeds 50k tokens
  keep_recent: 10                  # Always keep N most recent messages
  summary_model: "gpt-4o-mini"     # Model used for summarization
```

### When Triggered

Typical example:

```
Initial state:
- Message 1: "Tell me about X"
- Message 2: "Explain Y"
- ...
- Message 25: "Now how does Z work?"
- Total: 52,000 tokens

Compression applied:
- Summary: "User asked about X, Y, and other topics"
- Message 24: (kept as-is)
- Message 25: (kept as-is)
- Total: 35,000 tokens
```

### Cost Trade-off

Compression incurs a small cost (summarization LLM call) but saves tokens on main LLM:

```
Without compression:
- Request tokens: 50,000
- Cost: 50,000 × $0.005 = $0.25

With compression:
- Summarization: 5,000 → 500 tokens = $0.0025
- Request tokens: 35,000 (with summary)
- Cost: 35,000 × $0.005 + $0.0025 = $0.1775
- Savings: 29%
```

## A/B Testing

Run traffic-split experiments to compare model performance.

### How It Works

1. Define an experiment with two models
2. Specify traffic split (e.g., 20% variant, 80% control)
3. Requests are deterministically routed based on agent ID
4. Same agent always gets same variant (consistent experience)
5. Different agents see different variants (traffic split)

### Configuration

```yaml
experiments:
  - name: "test-gpt4o-vs-mini"
    enabled: true
    control_model: "gpt-4o"
    variant_model: "gpt-4o-mini"
    traffic_pct: 20                # 20% to variant, 80% to control

  - name: "claude-sonnet-vs-opus"
    enabled: true
    control_model: "claude-opus-4-6"
    variant_model: "claude-sonnet-4-5-20250929"
    traffic_pct: 50                # 50/50 split
```

### Checking Variant Assignment

```bash
# Check which variant an agent is assigned to
agix experiment check code-reviewer gpt-4o
# Output: code-reviewer → variant (gpt-4o-mini)

agix experiment check docs-writer gpt-4o
# Output: docs-writer → control (gpt-4o)
```

### Deterministic Assignment

Assignment is based on hash(agent_name + model):

```
Same agent + model combination = always same variant
Different agent = potentially different variant
```

### Analysis Pattern

Track experiment results manually:

1. Run experiment for N days
2. Export costs and logs:
   ```bash
   agix logs --agent code-reviewer -n 1000 | grep "gpt-4o"
   agix logs --agent docs-writer -n 1000 | grep "gpt-4o"
   ```
3. Compare:
   - Cost differences
   - Quality (manual review or quality gate warnings)
   - Speed (duration_ms from logs)

### Real-World Example

Testing gpt-4o vs gpt-4o-mini for code review:

```yaml
experiments:
  - name: "code-review-cost-test"
    enabled: true
    control_model: "gpt-4o"           # Baseline (expensive)
    variant_model: "gpt-4o-mini"      # Cheaper option
    traffic_pct: 50                   # 50% of agents try mini

# After 1 week:
# - Mini variant agents: 10,000 reviews, cost=$45, quality=98%
# - Full variant agents: 10,000 reviews, cost=$150, quality=99%
#
# Decision: 70% quality improvement not worth 3x cost
# → Switch all code reviewers to gpt-4o-mini
```

## Combining Features

### Example: Cost Optimization Pipeline

```yaml
# 1. Smart routing for cheap simple requests
routing:
  enabled: true
  tiers:
    simple:
      max_message_tokens: 500
      max_messages: 3
  model_map:
    gpt-4o:
      simple: "gpt-4o-mini"        # Cheap
      complex: "gpt-4o"            # Expensive

# 2. Cache common responses
cache:
  enabled: true
  similarity_threshold: 0.95
  ttl_minutes: 120                 # Keep longer (6 hours)

# 3. Compress old messages in long conversations
compression:
  enabled: true
  threshold_tokens: 40000
  keep_recent: 8
  summary_model: "gpt-4o-mini"

# 4. Test new cheaper models via A/B
experiments:
  - name: "all-mini-experiment"
    enabled: true
    control_model: "gpt-4o"
    variant_model: "gpt-4o-mini"
    traffic_pct: 30                # 30% try the cheaper option
```

**Result**: 40-60% cost reduction while maintaining quality.

## Best Practices

1. **Start with smart routing** — easiest win, lowest risk
2. **Monitor cache hit rates** — optimize `similarity_threshold` based on hits
3. **Test compression** — verify summaries are still useful before production
4. **A/B test models** — always validate cheaper models work for your use case
5. **Combine features** — routing + caching + compression for maximum savings
6. **Track quality** — enable quality gate to catch issues early
