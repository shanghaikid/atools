# 费用追踪与预算管理

## 概述

agix 自动追踪每一个 LLM 请求，提取 Token 用量，并根据提供商定价计算费用。这使得按 Agent 执行预算控制和详细的费用分析成为可能。

## 费用追踪的工作原理

### 请求流程

当 Agent 通过 agix 发送请求时：

1. Agent 发送 `POST /v1/chat/completions`，可附带可选的 `X-Agent-Name` 请求头
2. 代理将请求转发至上游 LLM 提供商
3. 响应返回时携带 Token 计数（`prompt_tokens`、`completion_tokens`）
4. 代理使用定价表计算费用：
   - 费用 = (prompt_tokens × 输入单价) + (completion_tokens × 输出单价)
5. 记录写入 SQLite/PostgreSQL
6. 响应头中包含费用信息

### 响应头

每个响应都包含以下请求头：

```
X-Cost-USD: 0.015          # 费用（美元）
X-Input-Tokens: 120        # 输入 Token 数
X-Output-Tokens: 45        # 输出 Token 数
X-Trace-ID: trace-abc123   # 用于可观测性追踪
```

### 数据存储

所有请求均持久化至数据库：

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

## 查看费用

### 整体统计

```bash
# 今日费用
agix stats

# 最近 7 天
agix stats --period 7d

# 指定月份（YYYY-MM 格式）
agix stats --period 2026-01

# JSON 格式输出
agix stats --format json
```

示例输出：
```
Total requests:  245
Total input:     18,450 tokens
Total output:    8,920 tokens
Total cost:      $12.45
Avg cost/request: $0.051
```

### 按 Agent 分类统计

```bash
# 按 Agent 统计费用
agix stats --group-by agent

# 按模型统计费用
agix stats --group-by model

# 按日统计费用（适合生成图表）
agix stats --group-by day
```

### 请求日志

```bash
# 最近 20 条请求
agix logs

# 最近 100 条请求
agix logs -n 100

# 实时追踪
agix logs --tail

# 按 Agent 过滤
agix logs --agent code-reviewer
```

日志列说明：
- 时间戳
- Agent 名称
- 使用的模型
- 输入/输出 Token 数
- 费用（美元）
- 响应时间（毫秒）

## 预算管理

### 设置预算

在 `~/.agix/config.yaml` 中配置预算：

```yaml
budgets:
  code-reviewer:
    daily_limit_usd: 10.0
    monthly_limit_usd: 200.0
    alert_at_percent: 80      # 消耗达 80% 时发出警告

  docs-writer:
    daily_limit_usd: 5.0
    monthly_limit_usd: 100.0
    alert_at_percent: 75
```

也可以使用 CLI：

```bash
# 同时设置日限额和月限额
agix budget set code-reviewer -d 10.0 -m 200.0

# 仅设置日限额
agix budget set code-reviewer -d 5.0

# 查看预算
agix budget

# 移除预算
agix budget remove code-reviewer
```

### 预算触发行为

当 Agent 达到预算上限时：

1. 请求到达代理
2. 代理检查：当前消费 vs 日限额 + 月限额
3. 若超出预算：返回 `429 Too Many Requests`
4. 响应中包含 `Retry-After` 请求头，表示需等待的秒数

错误响应示例：

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

### 故障开放安全机制

若数据库在预算检查期间不可用：

- 请求将**被放行**（故障开放）
- 费用在数据库恢复后补充记录
- 下次请求时重新进行预算检查

这确保了数据库的临时故障不会阻塞 Agent 运行。

## 费用优化模式

### 模式一：监控高消费 Agent

```bash
# 找出消费较高的 Agent
agix stats --group-by agent

# 深入查看特定 Agent
agix logs --agent expensive-agent -n 50
agix stats --agent expensive-agent --period 7d
```

**操作建议**：设置日限额以控制费用：

```bash
agix budget set expensive-agent -d 50.0
```

### 模式二：追踪模型费用

```bash
# 哪些模型费用最高？
agix stats --group-by model

# 分别追踪 gpt-4o 与 gpt-4o-mini 的用量
agix logs --model gpt-4o
agix logs --model gpt-4o-mini
```

**操作建议**：启用智能路由，对简单请求自动使用更廉价的模型（参见智能路由指南）。

### 模式三：每日费用趋势

```bash
# 导出每日费用
agix stats --group-by day --format json > daily_costs.json

# 可视化费用趋势
agix stats --group-by day
```

### 模式四：预算告警

配置 Webhook 通知，在消费达到阈值时触发：

```yaml
budgets:
  code-reviewer:
    daily_limit_usd: 10.0
    alert_at_percent: 80  # 消费达 $8.00 时告警

webhooks:
  definitions:
    budget-alert:
      secret: "webhook_secret"
      model: "gpt-4o-mini"
      prompt_template: |
        Agent <AgentName> has reached <SpentPercent>% of daily budget.
        Spent: $<Spent> / $<Limit>
      callback_url: "https://api.example.com/alerts/budget"
```

## 定价表

agix 内置以下模型的定价：

### OpenAI
- gpt-5.2、gpt-5.1、gpt-5
- gpt-5-mini、gpt-5-nano
- gpt-4.1、gpt-4.1-mini、gpt-4.1-nano
- gpt-4o、gpt-4o-mini
- o1、o3、o3-mini、o4-mini

### Anthropic
- claude-opus-4-6
- claude-sonnet-4-5-20250929
- claude-haiku-4-5-20251001
- claude-3-5-haiku-20241022

### DeepSeek
- deepseek-chat
- deepseek-reasoner

**注意**：带版本号的模型（如 `gpt-4o-2024-08-06`）通过最长前缀匹配与定价表进行关联。

## 常见问题

### Q：为什么我的费用计算结果与 LLM 提供商的账单不一致？

**A**：agix 使用模型列表中的实时定价。提供商账单可能包含：
- 批量折扣
- 批处理费率
- 计费周期内发生的不同定价变更
- 四舍五入差异

可使用 `agix stats --format json` 导出原始数据，与提供商账单对比。

### Q：可以按 Token 数而非美元设置预算吗？

**A**：目前预算仅支持美元单位。如需基于 Token 的限制，可使用频率限制：

```yaml
rate_limits:
  expensive-agent:
    requests_per_hour: 10  # 限制每小时最多 10 次请求
```

### Q：如何将费用导出到电子表格？

**A**：使用 CSV 导出：

```bash
agix export --format csv -o costs.csv
agix export --format csv --period 2026-01 -o january_costs.csv
```

在 Excel 或 Google Sheets 中打开进行分析。

### Q：预算可以在指定时间重置吗？

**A**：预算在 UTC 午夜重置。如需按其他时区重置：

1. 在应用中创建带有不同 UTC 偏移名称的多个 Agent
2. 或通过 API 手动协调重置（设置带 TTL 的会话覆盖）

## 最佳实践

1. **为每个 Agent 设置预算** — 防止单个 Agent 失控造成费用超支
2. **监控每日趋势** — 每周执行 `agix stats --group-by day`
3. **使用频率限制** — 将频率限制与预算配合使用
4. **启用审计日志** — 追踪工具调用的内容与时间
5. **配置告警** — 在预算消耗达到 80% 时通过 Webhook 发送通知
6. **定期导出** — 保存 CSV 导出记录，用于财务报告
