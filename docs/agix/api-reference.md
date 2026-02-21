---
title: HTTP API 参考
---

# HTTP API 参考

agix 提供两类 HTTP 接口：
- **代理接口**（`/v1/*`）：OpenAI 兼容的 LLM 请求入口
- **Dashboard API**（`/api/*`）：统计数据查询接口，供 Web 控制台使用

---

## 请求头

所有发往 agix 的请求都可以附带以下自定义请求头：

| 请求头 | 说明 |
|---|---|
| `X-Agent-Name` | Agent 标识符，启用后可按 Agent 追踪成本、执行预算控制和工具权限过滤 |
| `X-Session-ID` | Session ID，用于获取该 Session 的配置覆盖（模型、temperature 等） |
| `X-Force-Model` | 设置任意非空值可跳过智能路由，强制使用请求中指定的模型 |
| `X-Webhook-Signature` | Webhook 请求的 HMAC-SHA256 签名，格式：`sha256=HEX` |

---

## 响应头

agix 在每次响应中附加以下追踪和状态头：

### 成本追踪

| 响应头 | 示例值 | 说明 |
|---|---|---|
| `X-Cost-USD` | `0.002340` | 本次请求的计算成本（美元，6 位小数） |
| `X-Input-Tokens` | `1024` | 本次请求的输入 Token 数量 |
| `X-Output-Tokens` | `256` | 本次请求的输出 Token 数量 |

### 预算状态

当 Agent 配置了预算时，每次请求会附带当前预算使用率：

| 响应头 | 示例值 | 说明 |
|---|---|---|
| `X-Budget-Daily-Percent` | `73.5` | 今日预算使用百分比 |
| `X-Budget-Monthly-Percent` | `41.2` | 本月预算使用百分比 |

### 可观测性

| 响应头 | 示例值 | 说明 |
|---|---|---|
| `X-Trace-ID` | `abc123ef` | 请求追踪 ID（仅当 tracing 启用时返回） |
| `X-Cache` | `HIT` / `MISS` | 语义缓存是否命中（仅非流式请求） |

### 安全与质量

| 响应头 | 示例值 | 说明 |
|---|---|---|
| `X-Firewall-Warning` | `pii_detected` | 防火墙警告规则名称（可多个，每条独立一个 Header） |
| `X-Quality-Warning` | `empty_response` | Quality Gate 检测到的问题描述 |
| `X-Response-Policy` | `email_mask, truncated` | 已应用的响应策略（逗号分隔） |
| `Retry-After` | `60` | 触发限流（429）时需等待的秒数 |

---

## 代理接口

### POST /v1/chat/completions

OpenAI 兼容的聊天补全接口，agix 的核心代理入口。

**请求体**（OpenAI 格式）：

```json
{
  "model": "gpt-4o",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello!"}
  ],
  "stream": false
}
```

**请求体字段**：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `model` | string | ✅ | 模型名称，agix 据此自动判断上游服务商 |
| `messages` | array | ✅ | 对话消息列表（OpenAI 格式） |
| `stream` | boolean | | 是否流式输出（SSE），默认 `false` |
| 其他字段 | — | | `temperature`、`max_tokens` 等均透明透传 |

**响应**：上游 LLM 的原始响应，附加追踪 Header。

**状态码**：

| 状态码 | 说明 |
|---|---|
| `200` | 请求成功 |
| `400` | 请求体格式错误（JSON 非法、缺少 model 字段等） |
| `403` | 防火墙拦截（prompt injection、PII 等） |
| `422` | Quality Gate 拒绝响应（空响应、格式不符等） |
| `429` | 限流或预算超限（查看 `Retry-After` 响应头） |
| `502` | 上游服务商请求失败 |

**示例**：

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Agent-Name: my-agent" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

---

### GET /v1/models

列出 agix 支持的所有模型及其所属服务商。

**响应示例**：

```json
{
  "object": "list",
  "data": [
    {"id": "gpt-4o", "object": "model", "owned_by": "openai"},
    {"id": "claude-sonnet-4-6", "object": "model", "owned_by": "anthropic"},
    {"id": "deepseek-chat", "object": "model", "owned_by": "deepseek"}
  ]
}
```

---

### GET /health

健康检查接口，用于负载均衡或 readiness probe。

**响应**：

```json
{"status": "ok"}
```

---

## Sessions API

Session Override 允许按 Session ID 动态覆盖请求参数（模型、temperature、max_tokens），无需修改 Agent 代码。需在配置文件中启用 `session_overrides`。

### GET /v1/sessions/{id}

获取指定 Session 的当前覆盖配置。

**路径参数**：`{id}` — Session ID

**成功响应（200）**：

```json
{
  "session_id": "sess-abc123",
  "agent_name": "code-reviewer",
  "model": "gpt-4o-mini",
  "temperature": 0.2,
  "max_tokens": 2048,
  "expires_at": "2026-02-22T10:00:00Z"
}
```

**字段说明**：

| 字段 | 说明 |
|---|---|
| `session_id` | Session 唯一标识 |
| `agent_name` | 关联的 Agent 名称 |
| `model` | 覆盖的模型名（空表示不覆盖） |
| `temperature` | 覆盖的采样温度（null 表示不覆盖） |
| `max_tokens` | 覆盖的最大 Token 数（null 表示不覆盖） |
| `expires_at` | Session 过期时间（UTC） |

**错误响应**：

- `404` — Session 不存在或已过期
- `404` — Session Override 功能未启用

---

### PUT /v1/sessions/{id}

创建或更新 Session 覆盖配置（Upsert）。

**路径参数**：`{id}` — Session ID

**请求体**：

```json
{
  "agent_name": "code-reviewer",
  "model": "gpt-4o-mini",
  "temperature": 0.2,
  "max_tokens": 2048,
  "expires_at": "2026-02-22T10:00:00Z"
}
```

所有字段均可选。`expires_at` 省略时使用配置的 `default_ttl`（默认 1 小时）。

**成功响应（200）**：

```json
{"status": "ok", "session_id": "sess-abc123"}
```

**使用示例**：

```bash
# 为某个 Session 临时切换模型
curl -X PUT http://localhost:8080/v1/sessions/sess-abc123 \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "temperature": 0.5,
    "expires_at": "2026-02-22T12:00:00Z"
  }'
```

之后，携带 `X-Session-ID: sess-abc123` 的请求将自动使用 `gpt-4o-mini`：

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "X-Session-ID: sess-abc123" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o", "messages": [...]}'
# ↑ model 字段被 Session 覆盖，实际使用 gpt-4o-mini
```

---

### DELETE /v1/sessions/{id}

删除 Session 覆盖配置。

**成功响应（200）**：

```json
{"status": "deleted", "session_id": "sess-abc123"}
```

---

## Webhooks API

### POST /v1/webhooks/{name}

触发一个命名 Webhook：验证 HMAC 签名 → 渲染 Prompt 模板 → 调用 LLM → 执行回调（异步）。

**路径参数**：`{name}` — Webhook 名称（在配置文件 `webhooks.definitions` 中定义）

**请求头**：
- `X-Webhook-Signature: sha256=HEX` — 必填，HMAC-SHA256 签名

**签名计算方式**：

```bash
echo -n '请求体内容' | openssl dgst -sha256 -hmac "your-secret" -hex
# 在 Header 中使用：sha256=计算出的hex值
```

**请求体**：任意 JSON 或文本（透传给 Prompt 模板作为 `{{.Payload}}`）

**成功响应（202 Accepted）**：

```json
{
  "execution_id": 42,
  "status": "pending"
}
```

Webhook 异步执行，可通过 `agix webhook history` 查询执行结果。

**错误响应**：

| 状态码 | 说明 |
|---|---|
| `401` | HMAC 签名校验失败 |
| `404` | Webhook 未找到或功能未启用 |
| `405` | 不支持的请求方法（仅 POST） |

**配置示例**：

```yaml
# config.yaml
webhooks:
  definitions:
    github-pr-review:
      secret: "your-hmac-secret"
      prompt_template: "Review this PR: {{.Payload}}"
      model: "gpt-4o"
      callback_url: "https://your-app.com/webhook-result"
```

**Prompt 模板变量**：

| 变量 | 说明 |
|---|---|
| `{{.Payload}}` | 原始请求体内容（字符串） |

---

## Dashboard API

以下接口供 Web 控制台（`/dashboard/`）调用，也可直接通过 HTTP 查询数据。

### GET /api/stats

获取今日汇总统计数据。

**响应示例**：

```json
{
  "total_requests": 1204,
  "total_cost_usd": 3.42,
  "total_input_tokens": 845230,
  "total_output_tokens": 120450
}
```

---

### GET /api/agents

获取最近 30 天各 Agent 的统计数据。

**响应示例**：

```json
[
  {
    "agent_name": "code-reviewer",
    "request_count": 430,
    "total_cost_usd": 1.23,
    "total_input_tokens": 320000,
    "total_output_tokens": 45000
  }
]
```

---

### GET /api/budgets

获取所有 Agent 的预算配置和当前消费情况。

**响应示例**：

```json
{
  "code-reviewer": {
    "daily_limit_usd": 10.0,
    "monthly_limit_usd": 200.0,
    "daily_spend": 3.42,
    "monthly_spend": 78.90
  }
}
```

---

### GET /api/costs/daily

获取最近 30 天每日成本数据（用于折线图）。

**响应示例**：

```json
[
  {"date": "2026-02-20", "cost_usd": 2.10},
  {"date": "2026-02-21", "cost_usd": 3.42}
]
```

---

### GET /api/logs

获取最近的请求日志记录（最新 100 条）。

**响应示例**：

```json
[
  {
    "id": 1234,
    "timestamp": "2026-02-22T08:30:00Z",
    "agent_name": "code-reviewer",
    "model": "gpt-4o",
    "provider": "openai",
    "input_tokens": 1024,
    "output_tokens": 256,
    "cost_usd": 0.002340,
    "duration_ms": 1423,
    "status_code": 200
  }
]
```

---

## 完整请求流程

一次典型请求通过 agix 的处理顺序（中间件管道）：

```
请求到达
  │
  ├─ 限流检查 (X-Agent-Name)           → 超限返回 429 + Retry-After
  ├─ 预算检查 (daily/monthly)          → 超限返回 429
  ├─ Session Override 应用 (X-Session-ID)
  ├─ 防火墙扫描                        → 拦截返回 403，警告加 X-Firewall-Warning
  ├─ Prompt 模板注入
  ├─ 语义缓存查找 (非流式)             → 命中返回 200 + X-Cache: HIT
  ├─ 智能路由 (非 X-Force-Model)
  ├─ A/B 测试路由
  ├─ 上下文压缩
  │
  ├─[有工具] MCP 工具注入 → 工具调用循环
  └─[无工具] 上游请求 → Failover → Quality Gate
                                       → 响应策略 (X-Response-Policy)
                                       → 返回 + 追踪 Header
```
