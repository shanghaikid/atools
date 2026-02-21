# CLI 命令参考

## `agix init`

创建默认配置文件 `~/.agix/config.yaml`。

```bash
agix init
```

## `agix start`

启动反向代理服务器。

```bash
agix start              # 使用配置文件中的端口
agix start --port 9090  # 指定端口
```

## `agix stats`

查看用量统计。

```bash
agix stats                     # 今日总览
agix stats --by agent          # 按 Agent 分组
agix stats --by model          # 按模型分组
agix stats --by day            # 按天统计
agix stats --period 2026-01    # 指定月份
```

## `agix doctor`

运行全套健康检查，验证配置和依赖是否就绪。

```bash
agix doctor
```

### 输出格式

每项检查结果以 `PASS` / `WARN` / `FAIL` 标识开头：

```
  agix doctor

  PASS  Config file: /Users/you/.agix/config.yaml permissions OK (600)
  PASS  API keys: 2/2 valid
         openai: valid
         anthropic: valid
  PASS  Budgets: 3 agent(s) configured OK
  PASS  Firewall: 2 rule(s) valid
  WARN  Database: /Users/you/.agix/agix.db does not exist (will be created on first start)

  All checks passed!
```

若有检查失败，退出码为失败项数量（`> 0`）；全部通过则退出码为 `0`。

### 检查项说明

| 检查项 | 说明 | PASS | WARN | FAIL |
|--------|------|------|------|------|
| **Config file permissions** | 验证配置文件权限是否为 `0600`（含 API 密钥，不应被其他用户读取） | 权限为 `0600` | 权限过宽（组或其他用户可读） | 无法读取文件元信息 |
| **API key validity** | 向各 provider 发起轻量请求（`GET /models`）验证密钥有效性；OpenAI/DeepSeek 使用 `Authorization: Bearer`，Anthropic 使用 `x-api-key` | 所有已配置密钥有效 | 未配置任何 provider | 存在无效密钥（HTTP 401/403） |
| **Budget configuration** | 验证预算规则逻辑合理性：`daily ≤ monthly`，`alert_at_percent` 在 `[1, 100]` 范围内 | 所有规则合法 | 存在不合理规则 | — |
| **Firewall rules** | 编译每条自定义正则，验证 `action` 字段为 `block`/`warn`/`log` 之一 | 全部规则合法 | — | 存在非法正则或未知 action |
| **Database connectivity** | SQLite：检查文件存在性并运行 `PRAGMA integrity_check`；PostgreSQL：Ping + `SELECT version()` | 数据库健康 | SQLite 文件尚不存在（首次启动时自动创建） | 连接失败或完整性异常 |

## `agix logs`

查看请求日志。

```bash
agix logs                          # 最近 20 条
agix logs -n 100                   # 最近 100 条
agix logs --agent code-reviewer    # 按 Agent 筛选（-a 简写）
agix logs --tail                   # 实时追踪（500ms 轮询，-t 简写）
```

### 参数

| 参数 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| `--limit` | `-n` | `20` | 显示的记录条数 |
| `--agent` | `-a` | （全部） | 按 Agent 名称筛选 |
| `--tail` | `-t` | `false` | 实时追踪新请求 |

### 输出列说明

```
Recent Requests

 TIME               AGENT            MODEL                      INPUT    OUTPUT        COST   LATENCY  STATUS
 02-22 14:30:01     code-reviewer    claude-sonnet-4-6           1.2K     0.3K      $0.0042     312ms    200
 02-22 14:29:45     docs-writer      gpt-4o                      0.8K     0.5K      $0.0031     198ms    200
```

| 列 | 说明 |
|----|------|
| TIME | 请求时间（`月-日 时:分:秒`） |
| AGENT | Agent 名称（来自 `X-Agent-Name` 请求头，超 15 字符截断） |
| MODEL | 使用的模型名（超 25 字符截断） |
| INPUT | 输入 token 数（自动换算为 K） |
| OUTPUT | 输出 token 数（自动换算为 K） |
| COST | 本次请求费用（USD） |
| LATENCY | 请求延迟（毫秒） |
| STATUS | HTTP 状态码（200 绿色，4xx/5xx 红色） |

### `--tail` 实时模式

`--tail` 启动后每 500ms 轮询一次数据库，将新请求实时打印到终端（按 `Ctrl+C` 退出）。可与 `--agent` 组合使用，只追踪特定 Agent 的请求：

```bash
agix logs --tail --agent code-reviewer
```

## `agix budget`

管理 Agent 预算。

```bash
agix budget list                # 查看所有预算
agix budget set <agent> \
  --daily 10.0 \
  --monthly 200.0               # 设置预算
agix budget remove <agent>      # 移除预算
```

## `agix export`

导出用量数据。

```bash
agix export --format csv           # 导出 CSV
agix export --format json          # 导出 JSON
agix export --period 2026-01       # 指定月份
```

## `agix tools`

管理 MCP 工具。

```bash
agix tools list    # 列出所有可用的 MCP 工具
```

## `agix trace`

查看请求链路追踪（Trace）。

```bash
agix trace list                   # 列出最近 20 条 trace
agix trace list -n 10             # 列出最近 10 条
agix trace list -a my-agent       # 按 Agent 筛选
agix trace <trace-id>             # 查看某条 trace 的详细 Span 时间线
```

### `trace list` 输出

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

### `trace <trace-id>` 详情输出

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

### Span 字段说明

每个 Span 包含以下字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | Span 名称，标识处理阶段（见下表） |
| `duration_ms` | int64 | 该阶段耗时（毫秒） |
| `metadata` | object | 阶段相关的附加信息（JSON，可选） |

常见 Span 名称：

| Span 名称 | 说明 |
|-----------|------|
| `firewall_scan` | 防火墙规则匹配 |
| `cache_lookup` | 语义缓存查询 |
| `upstream_request` | 向上游 LLM provider 发起请求 |
| `tool_execution` | MCP 工具调用（每次循环一个） |
| `record_usage` | 写入用量记录到数据库 |

### 参数（`trace list`）

| 参数 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| `--number` | `-n` | `20` | 显示的 trace 条数 |
| `--agent` | `-a` | （全部） | 按 Agent 名称筛选 |

## `agix experiment`

管理 A/B 测试实验。

```bash
agix experiment list                         # 列出所有已配置的实验
agix experiment check <agent> <model>        # 查看某 Agent 的请求会命中哪个变体
```

### A/B 测试工作流

**第一步：在 `config.yaml` 中配置实验**

```yaml
experiments:
  - name: sonnet-vs-haiku
    enabled: true
    control_model: claude-sonnet-4-6     # 对照组（原始模型）
    variant_model: claude-haiku-4-5-20251001  # 实验组
    traffic_pct: 30                      # 30% 流量路由到实验组
```

**第二步：验证实验配置**

```bash
agix experiment list
```

输出：

```
 NAME              ENABLED  CONTROL                VARIANT                            TRAFFIC %
 sonnet-vs-haiku   yes      claude-sonnet-4-6      claude-haiku-4-5-20251001          30%
```

**第三步：确认 Agent 分配结果**

```bash
agix experiment check my-agent claude-sonnet-4-6
```

输出（命中实验组）：

```
Experiment: sonnet-vs-haiku
Variant:    variant
Model:      claude-haiku-4-5-20251001
```

输出（命中对照组）：

```
Experiment: sonnet-vs-haiku
Variant:    control
Model:      claude-sonnet-4-6
```

分配基于 `(agent_name, model)` 哈希确定性计算，同一 Agent 始终命中同一变体，保证实验的一致性。

**第四步：通过 `agix stats --by model` 对比两组的 token 用量和费用。**

### `experiment list` 列说明

| 列 | 说明 |
|----|------|
| NAME | 实验名称（`config.yaml` 中定义） |
| ENABLED | 是否启用（`yes` / `no`） |
| CONTROL | 对照组模型（请求原始 model 命中时使用） |
| VARIANT | 实验组模型 |
| TRAFFIC % | 路由到实验组的流量比例 |

### `experiment check` 输出字段

| 字段 | 说明 |
|------|------|
| Experiment | 匹配的实验名称 |
| Variant | 分配结果：`control`（对照组）或 `variant`（实验组） |
| Model | 实际使用的模型名称 |
