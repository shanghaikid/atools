# 配置文件

agix 的配置文件位于 `~/.agix/config.yaml`，由 `agix init` 创建。文件权限为 `0600`，目录权限为 `0700`。

## 完整配置示例

```yaml
# 代理监听端口
port: 8080

# LLM 提供商 API Key
keys:
  openai: "sk-..."
  anthropic: "sk-ant-..."

# SQLite 数据库路径
database: "/Users/you/.agix/agix.db"

# 日志级别
log_level: info

# 预算配置（可选）
budgets:
  code-reviewer:
    daily_limit_usd: 10.0
    monthly_limit_usd: 200.0
    alert_at_percent: 80
  docs-writer:
    daily_limit_usd: 5.0

# MCP 工具配置（可选）
tools:
  max_iterations: 10    # 每次请求最大工具执行轮数
  servers:
    filesystem:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    github:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-github"]
      env: ["GITHUB_TOKEN=ghp_xxx"]
  agents:
    code-reviewer:
      allow: ["read_file", "list_directory"]
    docs-writer:
      deny: ["write_file", "delete_file"]
```

## 字段说明

### 顶级字段

| 字段 | 类型 | 默认值 | 说明 | 验证规则 |
|------|------|--------|------|---------|
| `port` | int | `8080` | 代理监听端口 | 有效端口号（`agix start --port` 可覆盖） |
| `keys.openai` | string | - | OpenAI API Key | `agix doctor` 发送真实 HTTP 请求验证（401/403 为失败） |
| `keys.anthropic` | string | - | Anthropic API Key | 同上，使用 `x-api-key` 请求头 |
| `keys.deepseek` | string | - | DeepSeek API Key | 同上，使用 `Bearer` 请求头 |
| `database` | string | `~/.agix/agix.db` | SQLite 路径或 PostgreSQL URL | 前缀为 `postgres://` 或 `postgresql://` 时自动切换 PG 驱动；SQLite 时运行 `PRAGMA integrity_check` |
| `log_level` | string | `info` | 日志级别 | 无强制校验，推荐值：`debug` / `info` / `warn` / `error` |

### 预算配置

每个 Agent 可独立配置，超出限额返回 `429 Too Many Requests`。

| 字段 | 类型 | 默认值 | 说明 | 验证规则 |
|------|------|--------|------|---------|
| `daily_limit_usd` | float | - | 每日预算上限（美元） | 必须 ≤ `monthly_limit_usd`（`agix doctor` 检查） |
| `monthly_limit_usd` | float | - | 每月预算上限（美元） | 必须 ≥ `daily_limit_usd` |
| `alert_at_percent` | float | - | 预算告警阈值（百分比） | 必须在 `[1, 100]` 范围内 |
| `alert_webhook` | string | - | 告警 Webhook URL | 无强制校验，触发时发送 POST 请求 |

::: tip
`daily_limit_usd` 和 `monthly_limit_usd` 不要求同时配置，可只设其中一项。`agix doctor` 检查逻辑不满足时会输出 WARN，而不是 FAIL。
:::

### 工具配置

| 字段 | 类型 | 默认值 | 说明 | 验证规则 |
|------|------|--------|------|---------|
| `tools.max_iterations` | int | `10` | 每次请求的最大工具执行轮数 | 无强制校验，0 表示不限制 |
| `tools.servers.<name>.command` | string | - | MCP 服务器启动命令 | 无强制校验 |
| `tools.servers.<name>.args` | []string | - | 命令参数 | 无强制校验 |
| `tools.servers.<name>.env` | []string | - | 环境变量（格式：`KEY=value`） | 无强制校验 |
| `tools.agents.<name>.allow` | []string | - | 工具白名单 | 与 `deny` 互斥，不能同时配置 |
| `tools.agents.<name>.deny` | []string | - | 工具黑名单 | 与 `allow` 互斥，不能同时配置 |

::: warning 注意
`allow` 和 `deny` 不能同时配置。未列出的 Agent 默认可以使用所有工具。
:::

### 防火墙规则

| 字段 | 类型 | 默认值 | 说明 | 验证规则 |
|------|------|--------|------|---------|
| `firewall.enabled` | bool | `false` | 是否启用防火墙 | - |
| `firewall.rules[].name` | string | - | 规则名称 | - |
| `firewall.rules[].category` | string | - | 规则分类（如 `injection`、`pii`） | - |
| `firewall.rules[].pattern` | string | - | 正则表达式 | **必须是合法正则**，语法错误时 `agix doctor` 报 FAIL |
| `firewall.rules[].action` | string | - | 触发动作 | 必须为 `block`（返回 403）、`warn`（加响应头）、`log`（仅记录）之一 |

::: tip
内置规则始终生效：`injection_ignore`（block）、`injection_pretend`（warn）、`pii_ssn`（warn）、`pii_credit_card`（warn）。
:::

## 配置优先级与热重载

### 配置优先级

agix 的配置仅来源于两处，优先级从低到高：

```
配置文件 (~/.agix/config.yaml)
    ↓ 被以下内容覆盖
CLI flags (--port <N>)
```

**规则说明：**

- **配置文件**是所有配置的基础，缺失字段自动使用默认值（partial config 合法）。
- **`--port` flag**：`agix start --port 9000` 会在运行时覆盖配置文件中的 `port` 字段。
- **`--config` flag**：`agix start --config /path/to/custom.yaml` 指定替代配置文件路径（全局 flag，对所有子命令生效）。
- **环境变量**：agix **不支持**通过环境变量覆盖配置字段（`NO_COLOR` 除外，它控制终端着色输出）。

::: tip 最小配置
配置文件只需包含需要覆盖的字段，其余均使用默认值：

```yaml
# 最小有效配置：只设置 API Key
keys:
  anthropic: "sk-ant-..."
```
:::

### 热重载

**agix 不支持配置热重载。** 修改 `~/.agix/config.yaml` 后，需要重启 agix server 才能生效：

```bash
# 停止当前 server（Ctrl+C 或 kill）
agix start
```

目前无文件监听（inotify/kqueue）机制，无 SIGHUP 重载支持。

### 文件权限

| 路径 | 权限 | 原因 |
|------|------|------|
| `~/.agix/` | `0700` | 目录仅 owner 可访问 |
| `~/.agix/config.yaml` | `0600` | 文件含 API Key，仅 owner 可读写 |

`agix doctor` 会检查文件权限，若 `config.yaml` 对 group 或 others 可读，会输出 WARN。

### 运行 `agix doctor` 验证配置

```bash
agix doctor
```

| 检查项 | PASS 条件 | WARN 条件 | FAIL 条件 |
|--------|-----------|-----------|-----------|
| 文件权限 | 权限为 0600 | group/others 可读 | - |
| API Key 有效性 | 所有 Key 均通过 HTTP 验证 | 未配置任何 Key | 存在无效 Key（401/403） |
| 预算配置一致性 | daily ≤ monthly，alert_at_percent ∈ [1,100] | 检测到不一致 | - |
| 防火墙规则 | 所有正则合法，action 合法 | - | 正则语法错误或 action 非法 |
| 数据库连通性 | 数据库健康 | SQLite 文件尚不存在（首次启动前） | 连接/完整性检查失败 |
