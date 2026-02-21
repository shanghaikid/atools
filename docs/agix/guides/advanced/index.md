# 高级功能

agix 包含适用于专门用例的多项高级能力，每项功能均可独立启用。

## 功能列表

<div class="feature-grid">

### [系统提示词注入](./prompt-injection)

在代理层统一注入全局或按 Agent 的系统提示词，无需修改任何 Agent 代码。

→ [查看详情](./prompt-injection)

### [MCP 工具包](./mcp-bundle)

使用预打包的 MCP 服务器集合，一键为 Agent 添加文件、GitHub、代码审查等能力。

→ [查看详情](./mcp-bundle)

### [PostgreSQL 后端](./postgres)

将默认的 SQLite 替换为 PostgreSQL，支持多实例部署、高并发和生产级备份。

→ [查看详情](./postgres)

### [DeepSeek 提供商](./deepseek)

以 OpenAI 兼容格式使用 DeepSeek 模型，可作为低成本替代或故障转移目标。

→ [查看详情](./deepseek)

</div>

## 企业综合示例

以下示例展示了将多项高级功能组合使用的典型生产配置：

```yaml
# PostgreSQL 确保扩展性
database: "postgres://agix:password@prod-db.example.com:5432/agix?sslmode=require"

# 系统提示词注入统一安全策略
prompt_templates:
  enabled: true
  global: |
    你是企业 AI 助手。
    遵守 GDPR 和公司安全政策。
  agents:
    customer-support:
      template: "总是友好和专业。"

# MCP 工具包按需开放
bundles:
  - basic
  - github
  - code-review

tools:
  agents:
    developer-bot:
      allow: ["*"]
    customer-support:
      deny: ["delete_file", "modify_permissions", "git_commit"]

# DeepSeek 作为跨提供商故障转移
failover:
  chains:
    gpt-4o:
      - "gpt-4o-mini"
      - "claude-opus-4-6"
      - "deepseek-chat"

# 频率限制 + 预算
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

## 最佳实践

1. **谨慎使用系统提示词** — 仅用于关键策略，不用于一般性指导
2. **仅安装所需的工具包** — 减少复杂性和工具暴露面
3. **提前规划 PostgreSQL 迁移** — 最好在流量增长前完成
4. **跨提供商故障转移** — DeepSeek 作为低成本回退，Anthropic 保证质量
5. **测试提供商切换** — 各提供商的输出质量可能有所不同
6. **监控工具包更新** — 内置工具包可能新增或变更工具
