# tools · bundle

## `agix tools`

管理 MCP 工具。

### `agix tools list`

列出当前配置的所有 MCP 服务器上可用的工具：

```bash
agix tools list
```

输出示例：

```
 SERVER      TOOL              DESCRIPTION
 filesystem  read_file         Read the contents of a file
 filesystem  write_file        Write contents to a file
 filesystem  list_directory    List files in a directory
 github      create_issue      Create a new GitHub issue
 github      list_prs          List pull requests
```

工具来源于 `config.yaml` 中 `tools.servers` 配置的 MCP 服务器，代理启动时通过 JSON-RPC `tools/list` 接口发现。

## `agix bundle`

管理 MCP 工具包（Bundle）。Bundle 是预配置好的 MCP 服务器组合，一键安装即可为所有 Agent 提供一组工具。

### `bundle list`

列出所有已配置的工具包：

```bash
agix bundle list
```

### `bundle install <name>`

安装指定工具包，将对应的 MCP 服务器配置写入 `config.yaml`：

```bash
agix bundle install filesystem
agix bundle install github
```

### `bundle remove <name>`

从 `config.yaml` 中移除指定工具包的 MCP 服务器配置：

```bash
agix bundle remove github
```

### Agent 工具访问控制

可在 `config.yaml` 中为每个 Agent 配置工具访问控制（白名单或黑名单），无需修改 Agent 代码：

```yaml
tools:
  agents:
    code-reviewer:
      allow: ["read_file", "list_directory"]   # 只允许这些工具
    docs-writer:
      deny: ["write_file", "delete_file"]       # 禁止这些工具
    # 未列出的 Agent 可使用全部工具
```

详见[核心功能 - MCP 工具](/agix/features)。
