# CLI 命令参考

agix 提供完整的命令行界面，覆盖代理启动、统计查询、预算管理、工具管理和诊断调试。

## 命令速查表

| 命令 | 说明 |
|------|------|
| [`agix init`](./init-start) | 创建默认配置文件 |
| [`agix start`](./init-start) | 启动反向代理服务器 |
| [`agix stats`](./stats-logs) | 查看用量统计 |
| [`agix logs`](./stats-logs) | 查看 / 实时追踪请求日志 |
| [`agix export`](./stats-logs) | 导出用量数据（CSV / JSON） |
| [`agix budget`](./budget) | 管理 Agent 预算 |
| [`agix tools`](./tools-bundle) | 列出可用 MCP 工具 |
| [`agix bundle`](./tools-bundle) | 管理 MCP 工具包 |
| [`agix doctor`](./doctor) | 运行健康检查 |
| [`agix trace`](./trace) | 查看请求链路追踪 |
| [`agix experiment`](./experiment) | 管理 A/B 测试实验 |
| [`agix audit`](./advanced) | 查看安全审计日志 |
| [`agix session`](./advanced) | 管理会话级配置覆盖 |
| [`agix webhook`](./advanced) | 管理 Webhook |

## 全局选项

所有命令均支持以下全局选项：

| 选项 | 说明 |
|------|------|
| `--config <path>` | 指定配置文件路径（默认 `~/.agix/config.yaml`） |
| `--help` | 显示帮助信息 |
