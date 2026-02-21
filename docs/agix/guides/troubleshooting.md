# Troubleshooting & FAQs

## Common Issues

### Agent Gets 429 Too Many Requests

**Symptom**: Agent's requests are being rejected with 429 status

**Possible causes**:

1. **Budget exceeded**
   ```bash
   # Check budget status
   agix budget

   # Output shows agent over limit
   # Solution: Increase budget or wait for reset
   agix budget set agent-name -d 100.0
   ```

2. **Rate limit exceeded**
   ```yaml
   # Check config
   rate_limits:
     expensive-agent:
       requests_per_minute: 10    # Agent hitting this limit

   # Solution: Increase limit or implement backoff in agent
   ```

3. **Firewall blocking**
   ```bash
   # Check firewall blocks
   agix audit list --type firewall_block

   # Solution: Adjust firewall rules if legitimate requests are being blocked
   ```

**Quick fix**: Check in this order:
```bash
agix budget                        # 1. Budget exceeded?
agix stats --agent agent-name      # 2. Usage pattern?
agix audit list --agent agent-name # 3. Firewall blocks?
```

---

### High Cost Surprises

**Symptom**: Unexpected cost spike in daily stats

**Possible causes**:

1. **Agent in infinite retry loop**
   ```bash
   # Check request rate
   agix logs --tail --agent problematic-agent
   # Notice: 100s of requests in seconds?

   # Solution: Restart agent, set rate limit
   agix rate_limits set agent -m 5 --hour 100
   ```

2. **Premium model being used incorrectly**
   ```bash
   # Check model usage
   agix stats --group-by model

   # See gpt-4o being used for simple tasks?
   # Solution: Enable smart routing to auto-downgrade
   ```

3. **Cache disabled (cache hits at 0%)**
   ```bash
   # Check cache hit rate
   curl http://localhost:8080/api/stats

   # cache_hit_rate: 0?
   # Solution: Enable cache if appropriate for your workload
   ```

**Debugging**: Track cost progression
```bash
# Get hourly cost trend
agix stats --group-by day -p 1d    # Today so far
agix logs -n 500 | head -100       # Recent requests
```

---

### API Key Errors

**Symptom**: 401/403 Unauthorized from upstream provider

**Solution**:
```bash
# 1. Verify key is correct
vim ~/.agix/config.yaml
# Double-check: keys.openai, keys.anthropic, etc.

# 2. Run doctor to validate
agix doctor
# Output will show which key is invalid

# 3. Check key permissions
# OpenAI: key should have "read" and "write" permissions
# Anthropic: key format should be "sk-ant-..."
```

**Real example**:
```bash
$ agix doctor
...
API Key Validation
  ✗ OpenAI API key invalid (401)
  ✓ Anthropic API key valid
  ✓ DeepSeek API key valid

Solution: Update keys.openai in config.yaml
```

---

### Slow Responses

**Symptom**: Requests taking >5 seconds to complete

**Diagnosis**:
```bash
# 1. Check request traces
agix trace list | grep "Duration: [5-9][0-9][0-9][0-9]ms"

# 2. View detailed trace
agix trace trace-id-here

# 3. Check which span is slow:
#    - api.call > 3000ms → upstream provider is slow
#    - firewall.scan > 1000ms → regex patterns too complex
#    - store.write > 500ms → database slow
```

**Solutions by component**:

**If api.call is slow (>3s)**:
- Upstream provider is slow
- Check provider status page
- Consider failover to different model/provider

**If firewall.scan is slow (>1s)**:
- Custom regex patterns inefficient
- Simplify patterns or use "log" action for testing
- Disable rules not in production use

**If store.write is slow (>500ms)**:
- Database issue (SQLite lock or PostgreSQL slow)
- For SQLite: restart agix to unlock
- For PostgreSQL: check indexes

---

### Database Errors

**Symptom**: "database is locked" or connection timeouts

**For SQLite**:
```bash
# 1. Restart agix (clears lock)
pkill agix
sleep 2
agix start

# 2. Check integrity
agix doctor
# Should show: "✓ Database integrity check passed"

# 3. If failed, rebuild
cp ~/.agix/agix.db ~/.agix/agix.db.backup
rm ~/.agix/agix.db
agix start  # Creates new schema
```

**For PostgreSQL**:
```bash
# 1. Check connection
psql postgres://user:pass@host/agix -c "SELECT 1"

# 2. Check for locks
psql agix -c "SELECT * FROM pg_locks WHERE NOT granted;"

# 3. Kill stuck connections
psql agix -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE usename = 'agix';"

# 4. Restart agix
pkill agix
agix start
```

---

### MCP Tools Not Working

**Symptom**: Agent gets "tool not found" errors

**Diagnosis**:
```bash
# 1. List available tools
agix tools list

# 2. Check if tools are discovered
# If empty: MCP server didn't start properly

# 3. Check MCP server logs
agix doctor | grep "MCP Servers"
# Should show: "✓ filesystem server started (pid: 12345)"

# If failed, run doctor for details
```

**Common MCP issues**:

1. **npm package not installed**
   ```bash
   npm install -g @modelcontextprotocol/server-filesystem

   # Test command
   npx -y @modelcontextprotocol/server-filesystem /tmp
   # Should respond to initialize command
   ```

2. **Tool access denied by config**
   ```yaml
   # Check agent tool access
   tools:
     agents:
       agent-name:
         deny: ["tool_name"]  # Tool is in deny list!

   # Remove from deny list or add to allow list
   ```

3. **Tool requires environment variable**
   ```yaml
   tools:
     servers:
       github:
         command: "npx"
         args: ["-y", "@modelcontextprotocol/server-github"]
         env: ["GITHUB_TOKEN=ghp_xxx"]   # Missing or empty!
   ```

---

### Firewall Blocking Legitimate Requests

**Symptom**: Valid requests being rejected with "firewall blocked"

**Solution**:
```bash
# 1. Check what rule matched
agix audit list --type firewall_block -n 5

# 2. Review the rule
vim ~/.agix/config.yaml

# 3. Either:
#    a) Make rule more specific (better regex)
#    b) Change action from "block" to "warn" (for testing)
#    c) Remove rule if false positives

firewall:
  rules:
    - name: "problematic_rule"
      pattern: "..."
      action: "warn"    # Changed from "block"
```

**Example: Fixing over-aggressive injection detection**
```yaml
# Too aggressive: blocks "system" anywhere
- name: "injection_old"
  pattern: "system"
  action: "block"

# Better: only block "system prompt" together
- name: "injection_new"
  pattern: "(?i)system\\s+prompt"
  action: "block"
```

---

## Frequently Asked Questions

### Q: Can I route specific agents to specific models?

**A**: Use session overrides:

```python
# Create session for agent
session = requests.post("http://localhost:8080/v1/sessions/agent-session", json={
    "agent_name": "my-agent",
    "model": "gpt-4o-mini"  # Force this model
}).json()

# Use session in requests
client = OpenAI(
    base_url="http://localhost:8080/v1",
    extra_headers={"X-Session-ID": session["session_id"]}
)
```

Or use smart routing for automatic cost optimization.

---

### Q: How do I reduce costs?

**A**: In priority order:

1. **Enable smart routing** (easiest, 20-30% savings)
   ```yaml
   routing:
     enabled: true
     # Simple requests → cheaper models
   ```

2. **Enable semantic caching** (20-40% savings if applicable)
   ```yaml
   cache:
     enabled: true
   ```

3. **Switch to cheaper models** (30-50% savings)
   - gpt-4o-mini instead of gpt-4o
   - claude-haiku instead of claude-opus
   - deepseek-chat for general tasks

4. **Set rate limits** (prevent runaway costs)
   ```yaml
   rate_limits:
     agent: {requests_per_minute: 10}
   ```

---

### Q: Can I use multiple LLM providers?

**A**: Yes! Route models to different providers:

```yaml
keys:
  openai: "sk-..."
  anthropic: "sk-ant-..."
  deepseek: "sk-..."

# Requests automatically route:
# - gpt-4o → OpenAI
# - claude-opus-4-6 → Anthropic
# - deepseek-chat → DeepSeek
```

Use failover for automatic provider switching on errors.

---

### Q: How do I export data for reporting?

**A**:
```bash
# CSV export (Excel-friendly)
agix export --format csv -o costs.csv

# JSON export (for analysis)
agix export --format json -o costs.json

# Specific period
agix export --format csv --period 2026-01 -o january.csv

# Open in spreadsheet
open costs.csv
```

---

### Q: Can I reset budgets at a custom time?

**A**: Currently budgets reset at midnight UTC. Workaround:

```python
# Create session with custom model for part of day
import os
from datetime import datetime, timedelta

# If before noon UTC, use expensive model
# If after noon UTC, use cheap model
hour = datetime.utcnow().hour
model = "gpt-4o" if hour < 12 else "gpt-4o-mini"

# Create session with TTL until reset
session = requests.post("http://localhost:8080/v1/sessions/my-session", json={
    "agent_name": "my-agent",
    "model": model,
    "ttl": f"{24-hour}h"
}).json()
```

---

### Q: How do I monitor agix in production?

**A**: Setup:

1. **Enable all observability**
   ```yaml
   tracing:
     enabled: true
     sample_rate: 0.5

   audit:
     enabled: true

   dashboard:
     enabled: true
   ```

2. **Check daily**
   ```bash
   agix stats --period 1d       # Yesterday's costs
   agix logs -n 100              # Recent requests
   agix audit list -n 50         # Security events
   ```

3. **Setup alerting**
   ```bash
   # Via cron job that checks budget status
   agix budget | grep "%" | grep -E "[8-9][0-9]%|100%"
   # If matches, send alert
   ```

4. **Weekly review**
   ```bash
   agix stats --group-by agent  # By-agent breakdown
   agix export --format csv     # For reporting
   ```

---

### Q: What's the maximum request rate?

**A**: Depends on your hardware and database:

- **SQLite**: 100-500 requests/second (single server)
- **PostgreSQL**: 1000+ requests/second (with proper indexing)

To test:
```bash
# Monitor request throughput
agix logs --tail

# Check database performance
agix doctor
```

For high volume, use PostgreSQL with connection pooling.

---

### Q: Can I backup my data?

**A**:

**SQLite**:
```bash
# Simple file copy
cp ~/.agix/agix.db /backup/agix.db

# Or SQL export
sqlite3 ~/.agix/agix.db ".dump" > backup.sql
```

**PostgreSQL**:
```bash
# Backup
pg_dump agix > backup.sql

# Restore
psql agix < backup.sql
```

---

### Q: How do I troubleshoot quality gate issues?

**A**:
```bash
# Check for quality warnings in logs
agix logs -n 100 | grep -i "quality"

# Check trace for failed responses
agix trace list | head -20

# Look for on_empty/on_truncated/on_refusal triggers
# Review response_policy to see if truncation is happening
```

---

## Support

For additional help:

- **GitHub Issues**: Report bugs or request features
- **Documentation**: Check `/docs/agix/` for detailed guides
- **Health Check**: Run `agix doctor` to diagnose issues
- **Audit Trail**: Review `agix audit list` for security events
- **Logs**: Check `agix logs --tail` for real-time activity
