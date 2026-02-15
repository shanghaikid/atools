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

## `agix logs`

查看请求日志。

```bash
agix logs                          # 最近 50 条
agix logs -n 100                   # 最近 100 条
agix logs --agent code-reviewer    # 按 Agent 筛选
agix logs --tail                   # 实时追踪（500ms 轮询）
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
