# 核心功能

## Token 与成本追踪

agix 拦截每一次 LLM 请求的响应，提取 Token 用量信息并根据定价表计算成本，所有记录持久化到 SQLite 数据库。

**数据模型：**

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

**响应头注入：**

每个代理响应都会附加额外的 HTTP Header：

- `X-Cost-USD` —— 本次请求成本
- `X-Input-Tokens` —— 输入 Token 数
- `X-Output-Tokens` —— 输出 Token 数

## 预算控制

为每个 Agent 设置每日或每月的预算上限。当 Agent 的累计消费超过限额时，代理会返回 `429 Too Many Requests`。

```bash
agix budget set code-reviewer --daily 10.0 --monthly 200.0
agix budget list
agix budget remove code-reviewer
```

配置文件中：

```yaml
budgets:
  code-reviewer:
    daily_limit_usd: 10.0
    monthly_limit_usd: 200.0
    alert_at_percent: 80
```

::: tip 容错机制
预算检查采用 fail-open 策略：如果数据库查询失败，请求会被放行，而不是被拒绝。
:::

## MCP 工具注入

agix 可以作为 MCP 工具的集中管理器。Agent 不需要任何代码修改就能使用共享工具 —— 代理自动注入工具定义、执行工具调用、并将结果返回给 LLM。

### 工具执行流程

1. Agent 发送请求（不知道有工具存在）
2. 代理将 `stream` 强制设为 `false`
3. 代理注入工具定义到请求体
4. 发送给 LLM
5. 如果 LLM 返回 `tool_calls`：
   - 代理路由调用到对应的 MCP 服务器
   - 执行工具，收集结果
   - 追加到对话，重新发给 LLM
   - 重复，直到没有更多工具调用（最多 `max_iterations` 轮）
6. 最终返回干净的响应给 Agent（去除工具相关字段）

### 按 Agent 控制工具访问

```yaml
tools:
  agents:
    code-reviewer:
      allow: ["read_file", "list_directory"]    # 白名单
    docs-writer:
      deny: ["write_file", "delete_file"]       # 黑名单
    # 未列出的 Agent 可以使用所有工具
```

## 流式传输 (SSE)

对于不使用工具的 `"stream": true` 请求，代理会：

- 将每个 SSE chunk 立即转发给客户端
- 解析每行 `data:` 查找用量信息
- 流结束后记录 Token 总量

::: info 注意
当工具功能激活时，流式请求会被自动降级为非流式。Agent 会收到标准的非流式响应。
:::

## 多提供商路由

代理根据模型名称自动路由请求：

- `gpt-*`, `o1`, `o3*`, `o4*` → OpenAI API
- `claude-*` → Anthropic Messages API

对于 Anthropic 模型，代理会自动完成 OpenAI 格式到 Anthropic 格式的转换。

## 支持的模型

### OpenAI
gpt-5.2, gpt-5.1, gpt-5, gpt-5-mini, gpt-5-nano, gpt-4.1, gpt-4.1-mini, gpt-4.1-nano, gpt-4o, gpt-4o-mini, o1, o3, o3-mini, o4-mini

### Anthropic
claude-opus-4-6, claude-sonnet-4-5, claude-haiku-4-5, claude-3-5-haiku, claude-3-haiku

版本化的模型名称（如 `gpt-4o-2024-08-06`）通过最长前缀匹配定价表。
