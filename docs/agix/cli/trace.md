# trace

## `agix trace`

查看请求的详细链路追踪（Trace），了解每个处理阶段的耗时分布。

```bash
agix trace list                    # 列出最近 20 条 trace
agix trace list -n 10              # 列出最近 10 条
agix trace list -a my-agent        # 按 Agent 筛选
agix trace <trace-id>              # 查看某条 trace 的 Span 时间线
```

## `trace list`

列出最近的请求追踪记录。

```bash
agix trace list
agix trace list -n 10 -a code-reviewer
```

### 参数

| 参数 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| `--number` | `-n` | `20` | 显示的 trace 条数 |
| `--agent` | `-a` | （全部） | 按 Agent 名称筛选 |

### 输出

```
 TRACE ID                              AGENT           MODEL               SPANS  TIMESTAMP
 550e8400-e29b-41d4-a716-446655440000  code-reviewer   claude-sonnet-4-6   4      2026-02-22 14:30:01
 f47ac10b-58cc-4372-a567-0e02b2c3d479  docs-writer     gpt-4o              2      2026-02-22 14:29:45
```

| 列 | 说明 |
|----|------|
| TRACE ID | 唯一追踪 ID（UUID 格式），用于 `agix trace <id>` 查看详情 |
| AGENT | 发起请求的 Agent |
| MODEL | 使用的模型 |
| SPANS | 本次请求记录的 Span 数量 |
| TIMESTAMP | 请求时间 |

## `trace <trace-id>`

查看单条 trace 的完整 Span 时间线。

```bash
agix trace 550e8400-e29b-41d4-a716-446655440000
```

### 输出

```
Trace: 550e8400-e29b-41d4-a716-446655440000
Agent: code-reviewer
Model: claude-sonnet-4-6
Time:  2026-02-22T14:30:01Z

 #  SPAN              DURATION  DETAILS
 1  firewall_scan       12ms    {"rules_checked":3}
 2  cache_lookup         3ms    {"hit":false}
 3  upstream_request   285ms    {"provider":"anthropic","status":200}
 4  record_usage         2ms    {}
```

### Span 字段

每个 Span 包含以下字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | Span 名称，标识处理阶段 |
| `duration_ms` | int64 | 该阶段耗时（毫秒） |
| `metadata` | object | 阶段附加信息（JSON，可选） |

### 常见 Span 名称

| Span | 说明 |
|------|------|
| `firewall_scan` | 防火墙规则匹配 |
| `cache_lookup` | 语义缓存查询 |
| `upstream_request` | 向上游 LLM provider 发起请求 |
| `tool_execution` | MCP 工具调用（每次工具循环一个 Span） |
| `record_usage` | 写入用量记录到数据库 |

Trace 数据由代理在每次请求处理过程中自动记录，无需额外配置。
