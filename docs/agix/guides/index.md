# agix Comprehensive Guides

Welcome to the agix feature guides! These detailed how-to guides help you configure, use, and optimize agix for your specific use case.

## Guide Categories

### 💰 [Cost Tracking & Budget Management](./cost-tracking.md)

Understand how agix tracks costs, manages budgets, and helps you control spending.

**Topics covered:**
- How cost tracking works (token extraction, pricing calculation)
- Viewing costs with `agix stats` and `agix logs`
- Per-agent budget enforcement and limits
- Cost optimization patterns and best practices
- Troubleshooting cost discrepancies

**Best for:** Finance teams, cost-conscious deployments, multi-tenant setups

---

### 🧠 [Intelligence & Optimization](./intelligence-optimization.md)

Learn how to optimize requests, reduce costs, and improve performance with advanced features.

**Topics covered:**
- Smart routing: automatically use cheaper models for simple requests
- Semantic caching: cache responses for similar prompts
- Context compression: summarize old messages for long conversations
- A/B testing: traffic-split experiments to compare models

**Best for:** Cost optimization, performance tuning, experimentation

---

### 🔒 [Safety & Control](./safety-control.md)

Protect your system with prompt firewalls, response policies, and quality gates.

**Topics covered:**
- Prompt firewall: detect and block injection attempts
- Response policy: redact PII, enforce formats, truncate responses
- Quality gate: auto-retry empty or refused responses
- Session overrides: per-request configuration changes

**Best for:** Security-conscious teams, regulated industries, content filtering

---

### 📊 [Observability](./observability.md)

Monitor, debug, and understand what's happening in your proxy.

**Topics covered:**
- Request tracing: detailed per-span diagnostics
- Audit logging: security and tool call tracking
- Health checks: `agix doctor` for system diagnosis
- Metrics dashboard: web UI for cost visualization

**Best for:** DevOps, debugging, production monitoring

---

### 🚀 [Reliability & Scale](./reliability-scale.md)

Build reliable, scalable systems with failover, rate limiting, and webhooks.

**Topics covered:**
- Multi-provider failover: automatic fallback chains
- Rate limiting: per-agent request throttling
- Budget alerts: webhook notifications on spending thresholds
- Generic webhooks: receive events, process with LLM, fire callbacks

**Best for:** Production deployments, high-volume systems, event-driven workflows

---

### ⚙️ [Advanced Features](./advanced-features.md)

Deep dive into specialized capabilities for complex use cases.

**Topics covered:**
- System prompt injection: enforce policies globally or per-agent
- MCP tool bundles: pre-packaged tool sets for common workflows
- PostgreSQL backend: scalable alternative to SQLite
- DeepSeek provider: additional LLM provider support

**Best for:** Enterprise deployments, custom integrations, high-volume systems

---

### 🔧 [Troubleshooting & FAQs](./troubleshooting.md)

Quick solutions to common problems and answers to frequently asked questions.

**Topics covered:**
- Common issues: 429 errors, high costs, API key problems, slow responses
- Database troubleshooting: SQLite locks, PostgreSQL connection issues
- MCP tools not working: debugging tool discovery and access
- FAQs: cost reduction, multi-provider setup, data export, monitoring

**Best for:** Problem-solving, getting unstuck, learning best practices

---

## Getting Started Path

**New to agix?** Start here:

1. Read the [main README](../index.md) for overview
2. Follow [Quick Start](../quickstart.md) to get running
3. Review [Configuration](../config.md) to understand options
4. Pick your first guide based on your needs:
   - **Cost-conscious?** → [Cost Tracking](./cost-tracking.md)
   - **Want to save money?** → [Intelligence & Optimization](./intelligence-optimization.md)
   - **Need security?** → [Safety & Control](./safety-control.md)
   - **Running in production?** → [Observability](./observability.md) + [Reliability & Scale](./reliability-scale.md)

## Common Workflows

### Scenario 1: Reduce costs by 50%

1. Enable smart routing ([Intelligence & Optimization](./intelligence-optimization.md))
2. Enable semantic caching (same guide)
3. Set up budget alerts ([Reliability & Scale](./reliability-scale.md))
4. Monitor with dashboard ([Observability](./observability.md))

### Scenario 2: Secure multi-tenant setup

1. Enable prompt firewall ([Safety & Control](./safety-control.md))
2. Enable response policy with PII redaction (same guide)
3. Set per-agent tool access ([Advanced Features](./advanced-features.md))
4. Enable audit logging ([Observability](./observability.md))
5. Set rate limits ([Reliability & Scale](./reliability-scale.md))

### Scenario 3: Scale to high volume

1. Migrate to PostgreSQL ([Advanced Features](./advanced-features.md))
2. Set up multi-provider failover ([Reliability & Scale](./reliability-scale.md))
3. Enable tracing for monitoring ([Observability](./observability.md))
4. Configure webhooks for alerting (same guide)
5. Review [Troubleshooting](./troubleshooting.md) for production issues

### Scenario 4: Enterprise with policies

1. Inject system prompts ([Advanced Features](./advanced-features.md))
2. Enable firewall with custom rules ([Safety & Control](./safety-control.md))
3. Set per-agent budgets ([Cost Tracking](./cost-tracking.md))
4. Install MCP tool bundles ([Advanced Features](./advanced-features.md))
5. Monitor everything ([Observability](./observability.md))

## Feature Checklist

Use this to find which guides you need:

```
☐ Understand pricing and costs → Cost Tracking
☐ Reduce spending → Intelligence & Optimization
☐ Block prompt injection → Safety & Control
☐ Redact PII from responses → Safety & Control
☐ Auto-retry failed requests → Safety & Control
☐ Override config per-request → Safety & Control
☐ Debug slow requests → Observability
☐ Audit who called what tool → Observability
☐ Monitor system health → Observability
☐ Handle provider failures → Reliability & Scale
☐ Limit agent request rate → Reliability & Scale
☐ Alert on high spending → Reliability & Scale
☐ Integrate webhooks → Reliability & Scale
☐ Inject system prompts → Advanced Features
☐ Manage shared tools → Advanced Features
☐ Scale with PostgreSQL → Advanced Features
☐ Troubleshoot 429 errors → Troubleshooting
☐ Fix slow responses → Troubleshooting
```

## Tips for Success

1. **Start small** — Enable one feature at a time, test thoroughly
2. **Monitor impact** — Check `agix stats` daily to see effects
3. **Use rate limiting defensively** — Catch runaway agents early
4. **Set budget alerts** — Don't wait until agents hit limits
5. **Test firewall rules** — Use "log" action first to validate
6. **Review audit logs weekly** — Catch security issues early
7. **Run doctor regularly** — Catch config issues before they cause problems
8. **Backup data regularly** — Daily backups of SQLite or `pg_dump` for PostgreSQL

## Need Help?

- **Can't find what you need?** Check the [CLI reference](../cli.md)
- **Want to understand the architecture?** Read [agix/CLAUDE.md](https://github.com/ryjiang/agent-platform/tree/main/tools/agix#readme) for technical details
- **Having issues?** See [Troubleshooting](./troubleshooting.md) or run `agix doctor`
- **Want to contribute?** Check the GitHub repository

---

**Happy optimizing! 🚀**
