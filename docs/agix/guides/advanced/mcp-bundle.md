# MCP 工具包

工具包（Bundle）是适用于常见工作流的预打包 MCP 服务器集合，一行配置即可为 Agent 添加文件操作、GitHub 访问、代码审查等能力。

## 内置工具包

```bash
agix bundle list
```

| 名称 | 描述 | 包含工具 |
|---|---|---|
| `basic` | 文件操作 | 读文件、写文件、列目录 |
| `github` | GitHub 仓库访问 | 读取、搜索、提交 |
| `code-review` | 代码分析 | 静态分析、diff 查看 |
| `devops` | 基础设施工具 | 部署、监控查询 |
| `docs-writer` | 文档与发布 | 文档生成、发布工具 |

## 安装与查看

```bash
# 安装工具包
agix bundle install basic
agix bundle install github

# 查看工具包详情
agix bundle show github
# 输出：
# 包：github
# 描述：GitHub 仓库访问
# 服务器：
#   - github：GitHub 客户端（需要 GITHUB_TOKEN 环境变量）

# 移除工具包
agix bundle remove basic
```

## 配置工具包

在 `config.yaml` 中启用工具包：

```yaml
# 启用内置工具包
bundles:
  - basic
  - github

# 也可以配置自定义 MCP 服务器
tools:
  max_iterations: 10
  servers:
    custom-api:
      command: "npx"
      args: ["-y", "@company/custom-api-server"]
      env: ["API_KEY=your-key"]
```

## 工具访问控制

通过 `allow` 和 `deny` 列表精细控制每个 Agent 可以使用哪些工具：

```yaml
tools:
  servers:
    filesystem:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]

    github:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-github"]
      env: ["GITHUB_TOKEN=ghp_xxx"]

  agents:
    # 白名单：只允许指定工具
    code-reviewer:
      allow: ["read_file", "list_directory"]

    # 允许所有工具（可信 Agent）
    docs-writer:
      allow: ["*"]

    # 黑名单：禁止指定工具，其余全部允许
    security-agent:
      deny: ["delete_file", "modify_permissions"]

    # 未配置的 Agent 默认获得所有工具
```

## 工具调用流程

当 Agent 发送请求时，如果该 Agent 有可用工具，agix 自动进行工具循环：

```
Agent 发送请求（不含工具，不感知工具存在）
    ↓
agix 注入工具定义 → 强制 stream=false
    ↓
LLM 返回 tool_calls?
    ├─ 是 → agix 并发执行 MCP 工具调用
    │         ↓
    │       追加工具结果到对话
    │         ↓
    │       重新请求 LLM（最多 max_iterations 次）
    └─ 否 → 清除工具相关字段 → 返回给 Agent
```

Agent 始终看到干净的响应，无感知工具的存在。

## 真实示例：文档工作流

```bash
# 安装文档写作工具包
agix bundle install docs-writer
```

```yaml
bundles:
  - docs-writer    # 包含文件系统 + GitHub + 发布工具

tools:
  agents:
    docs-agent:
      allow: ["read_file", "write_file", "list_directory", "git_commit"]
```

现在 `docs-agent` 可以：
- 读取代码文件以提取示例
- 自动生成或更新文档
- 提交变更到 git

全部无需修改 Agent 代码。

## 注意事项

- 工具调用循环强制使用非流式模式，Agent 最终收到非流式响应
- `max_iterations` 默认为 10，防止无限循环
- 同一 MCP 服务器的调用通过 mutex 串行执行，不同服务器并发执行
- 工具执行失败时，错误信息会作为工具结果返回给 LLM，由 LLM 决定如何处理
