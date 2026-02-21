# 可靠性与扩展

## 概述

agix 包含用于处理提供商故障、控制请求速率、支出告警和 Webhook 的功能：

- **多提供商故障转移** — 提供商出错时自动进行备用链路转移
- **频率限制** — 按 Agent 的请求节流（请求/分钟和小时）
- **预算告警** — 支出达到阈值时发送 Webhook 通知
- **通用 Webhook** — 接收 Webhook、渲染模板、执行 LLM、触发回调

## 多提供商故障转移

故障转移在主要提供商失败时自动路由到备用模型。

### 工作原理

1. 为模型 X 的请求到达
2. 代理检查 X 的故障转移链
3. 尝试调用 X
4. 如果 X 失败（API 错误、超时等）：
   - 回退到第一个替代方案
   - 使用新模型重试
5. 如果替代方案失败：
   - 尝试链中的下一个
6. 如果全部失败：
   - 向 Agent 返回错误

### 配置

```yaml
failover:
  max_retries: 2                   # 最多重试 2 次

  chains:
    gpt-4o:
      - "gpt-4o-mini"              # 如果 4o 失败，尝试 mini
      - "gpt-35-turbo"             # 然后尝试 3.5 turbo

    claude-opus-4-6:
      - "claude-sonnet-4-5-20250929"
      - "claude-haiku-4-5-20251001"

    deepseek-chat:
      - "gpt-4o"                   # 回退到 OpenAI
```

### 真实示例：OpenAI 故障

场景：OpenAI API 宕机

```
gpt-4o 请求到达
  → OpenAI API 返回 503
  → 失败，尝试回退：gpt-4o-mini
  → OpenAI API 返回 503（仍然宕机）
  → 失败，尝试回退：gpt-35-turbo
  → OpenAI API 返回 503（仍然宕机）
  → 向 Agent 返回错误
```

使用跨提供商故障转移的更好设置：

```yaml
failover:
  chains:
    gpt-4o:
      - "gpt-4o-mini"              # 尝试更便宜的 OpenAI 模型
      - "claude-opus-4-6"          # 回退到 Anthropic
      - "deepseek-chat"            # 回退到 DeepSeek
```

现在：

```
gpt-4o 请求到达
  → OpenAI API 返回 503
  → 尝试 gpt-4o-mini → 也是 503
  → 尝试 claude-opus-4-6 → 200 OK ✓
  → 从 Anthropic 返回响应
```

### 成本影响

故障转移增加重试次数，增加 Token 用量：

```
场景：gpt-4o → gpt-4o-mini → claude-opus
- 请求 1 (gpt-4o)：5000 输入 Token → 失败
- 请求 2 (gpt-4o-mini)：5000 输入 Token → 失败
- 请求 3 (claude-opus)：5000 输入 Token → 成功
- 总计：15000 输入 Token（3 倍正常）
- 成本：3 倍正常成本
```

**建议**：设置 `max_retries: 1` 来限制重试成本。

## 频率限制

频率限制控制每个 Agent 可以发出的请求数量。

### 工作原理

1. 请求到达，带有 `X-Agent-Name` 请求头
2. 代理检查 Agent 的频率限制
3. 如果 Agent 在限制内：
   - 请求通过
   - 计数器增加
4. 如果 Agent 达到/超过限制：
   - 请求被拒绝，返回 429 状态
   - 响应包含 `Retry-After` 请求头

### 配置

```yaml
rate_limits:
  expensive-agent:
    requests_per_minute: 10
    requests_per_hour: 100

  default-agent:
    requests_per_minute: 30
    requests_per_hour: 500

  # 未列出的 Agent 无限制
```

### 响应请求头

频率限制时：

```
HTTP/1.1 429 Too Many Requests
Retry-After: 6                   # 等待 6 秒

{
  "error": {
    "message": "Rate limit exceeded for agent: expensive-agent (10 req/min)",
    "type": "rate_limit_exceeded"
  }
}
```

### 真实示例：防止失控 Agent

```yaml
# Agent 在死循环中调用 LLM
expensive-agent:
  requests_per_minute: 5           # 每分钟最多 5 个请求
  requests_per_hour: 100           # 每小时最多 100 个

# 不使用频率限制：
# - Agent 在 1 分钟内发送 1000 个请求
# - 代理处理所有请求
# - 成本：$150（如果每个请求成本 $0.15）
# - 用户在运行 1 小时后发现成本

# 使用频率限制：
# - Agent 发送 1000 个请求
# - 每分钟只允许 5 个
# - 其他返回 429 Retry-After: 12
# - Agent 可以实现退避
# - 每小时最高成本：$12.50
```

### 在 Agent 中实现退避

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
            # 等待 retry_after 秒
            wait_seconds = int(e.response.headers.get("Retry-After", 60))
            print(f"频率限制，等待 {wait_seconds} 秒...")
            time.sleep(wait_seconds)
        else:
            raise
```

## 预算告警

预算告警在 Agent 支出达到特定阈值时通过 Webhook 通知你。

### 工作原理

1. Agent 发出请求
2. 代理检查：当前支出 vs 日限制
3. 如果 spend_percent ≥ alert_threshold：
   - 触发 Webhook，带有告警数据
   - 包含 Agent 名称、支出金额、限制

### 配置

```yaml
budgets:
  code-reviewer:
    daily_limit_usd: 10.0
    monthly_limit_usd: 200.0
    alert_at_percent: 80         # 支出 80% 时告警（$8.00）

# Webhook 目标
webhooks:
  definitions:
    budget-alert:
      secret: "webhook_secret_key"
      model: "gpt-4o-mini"
      prompt_template: |
        告警：Agent <AgentName> 已达日预算的 <SpentPercent>%。

        当前支出：$<Spent>
        日限制：$<Limit>
        剩余：$<Remaining>

        时间：<Timestamp>

      callback_url: "https://slack.example.com/alerts"
```

### Webhook 签名

Webhook 使用 HMAC-SHA256 签名：

```
请求头：X-Webhook-Signature: sha256=<hex>
密钥：用于签名有效载荷
```

在 Webhook 处理器中验证：

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

## 通用 Webhook

通用 Webhook 允许你接收 HTTP 事件、用 LLM 处理，然后触发回调。

### 工作原理

1. 外部系统发送 HTTP POST 到 Webhook 端点
2. 代理接收 Webhook 有效载荷
3. 用有效载荷数据渲染模板
4. 发送提示词到 LLM（指定的模型）
5. 获取 LLM 响应
6. 使用 LLM 输出向 callback_url 发送回调

### 配置

```yaml
webhooks:
  enabled: true

  definitions:
    summarize-report:
      secret: "webhook_secret"
      model: "gpt-4o-mini"
      prompt_template: |
        用 3 个要点总结此报告：

        <Payload>

      callback_url: "https://api.example.com/callback"

    translate-content:
      secret: "webhook_secret"
      model: "gpt-4o"
      prompt_template: |
        翻译以下内容为西班牙语：

        <Payload>

      callback_url: "https://api.example.com/translated"
```

### 发送 Webhook

有效载荷文件 (`webhook-payload.json`)：

```json
{
  "title": "销售报告",
  "data": "Q1 收入增长 25%"
}
```

用 HMAC 签名发送 Webhook：

```bash
SECRET="webhook_secret"
PAYLOAD=$(cat webhook-payload.json)
SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$SECRET" -hex | sed 's/^.* //')

curl -X POST http://localhost:8080/v1/webhooks/summarize-report \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Signature: sha256=$SIGNATURE" \
  -d "$PAYLOAD"
```

### 真实示例：内容处理流水线

```yaml
webhooks:
  definitions:
    process-user-feedback:
      secret: "feedback_secret"
      model: "gpt-4o-mini"
      prompt_template: |
        将此客户反馈分类为积极、消极或中立。
        提取关键主题。
        建议一句话回应。

        反馈：
        <Payload>

        以 JSON 响应，键为：sentiment, topics, suggested_response

      callback_url: "https://api.example.com/feedback/processed"

# 用户通过网络表单提交反馈
# 网络应用向 agix 发送 Webhook
# agix 用 LLM 处理
# 结果发送到 CRM 系统
```

### 模板变量

可在 `prompt_template` 中使用：

| 变量 | 类型 | 示例 |
|------|------|------|
| `.Payload` | 字符串 | 来自 Webhook 的原始 JSON/文本 |
| `.Headers` | 映射 | 请求请求头 |
| `.Timestamp` | 字符串 | ISO 8601 时间戳 |
| `.WebhookName` | 字符串 | Webhook 定义名称 |

### Webhook 历史

```bash
# 查看最近的 Webhook 执行
agix webhook history

# 输出
# 时间戳            Webhook            状态    LLM 模型        延迟
# 2026-02-21 10:15   summarize-report   SUCCESS gpt-4o-mini    450ms
# 2026-02-21 10:16   process-feedback   SUCCESS gpt-4o         1200ms
# 2026-02-21 10:17   translate-content  ERROR   gpt-4o         (连接超时)
```

## 组合可靠性功能

### 示例：企业生产设置

```yaml
# 1. 跨提供商故障转移
failover:
  max_retries: 1                   # 限制重试成本
  chains:
    gpt-4o:
      - "gpt-4o-mini"              # 先尝试更便宜的模型
      - "claude-opus-4-6"          # 跨提供商回退

    claude-opus-4-6:
      - "claude-sonnet-4-5-20250929"  # 尝试更便宜的 Anthropic
      - "gpt-4o"                   # 跨提供商回退

# 2. 按 Agent 频率限制
rate_limits:
  critical-agent:
    requests_per_minute: 30
    requests_per_hour: 500

  batch-agent:
    requests_per_minute: 5
    requests_per_hour: 100

# 3. 预算执行 + 告警
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
        告警：Agent <AgentName> 已支出 <SpentPercent>%
      callback_url: "https://slack.example.com/alerts"

# 4. 详细的可观测性
audit:
  enabled: true

tracing:
  enabled: true
  sample_rate: 0.1                 # 采样 10% 的请求
```

## 最佳实践

1. **设置故障转移链** — 总是有备用提供商
2. **防御性使用频率限制** — 尽早捕获失控的 Agent
3. **设置预算告警较低** — 在 75-80% 时获得警告，而不是 95%
4. **实现 Webhook 退避** — 处理临时回调失败
5. **监控故障转移使用** — 高故障转移率表示提供商问题
6. **在低流量时测试故障转移** — 不要在生产环境中发现故障
7. **保持 Webhook 有效载荷较小** — 大有效载荷减慢处理速度
8. **验证 Webhook 签名** — 始终在处理前验证 HMAC
