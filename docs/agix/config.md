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

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `port` | int | `8080` | 代理监听端口 |
| `keys.openai` | string | - | OpenAI API Key |
| `keys.anthropic` | string | - | Anthropic API Key |
| `database` | string | `~/.agix/agix.db` | SQLite 数据库路径 |
| `log_level` | string | `info` | 日志级别 |

### 预算配置

| 字段 | 类型 | 说明 |
|------|------|------|
| `daily_limit_usd` | float | 每日预算上限（美元） |
| `monthly_limit_usd` | float | 每月预算上限（美元） |
| `alert_at_percent` | int | 预算告警阈值（百分比） |

### 工具配置

| 字段 | 类型 | 说明 |
|------|------|------|
| `tools.max_iterations` | int | 每次请求的最大工具执行轮数 |
| `tools.servers.<name>.command` | string | MCP 服务器启动命令 |
| `tools.servers.<name>.args` | []string | 命令参数 |
| `tools.servers.<name>.env` | []string | 环境变量 |
| `tools.agents.<name>.allow` | []string | 工具白名单 |
| `tools.agents.<name>.deny` | []string | 工具黑名单 |

::: warning 注意
`allow` 和 `deny` 不能同时配置。未列出的 Agent 默认可以使用所有工具。
:::
