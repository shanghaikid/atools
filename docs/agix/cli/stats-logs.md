# stats · logs · export

## `agix stats`

查看 token 用量与费用统计。

```bash
agix stats                     # 今日总览
agix stats --by agent          # 按 Agent 分组
agix stats --by model          # 按模型分组
agix stats --by day            # 按天统计
agix stats --period 2026-01    # 指定月份（YYYY-MM）
```

| 选项 | 说明 |
|------|------|
| `--by <group>` | 分组维度：`agent` / `model` / `day` |
| `--period <月份>` | 指定统计月份，格式 `YYYY-MM`（默认当月） |

## `agix logs`

查看请求日志，支持筛选和实时追踪。

```bash
agix logs                          # 最近 20 条
agix logs -n 100                   # 最近 100 条
agix logs --agent code-reviewer    # 按 Agent 筛选
agix logs --tail                   # 实时追踪（500ms 轮询）
agix logs --tail --agent mybot     # 实时追踪指定 Agent
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

`--tail` 每 500ms 轮询一次数据库，将新请求实时打印到终端（`Ctrl+C` 退出）。可与 `--agent` 组合使用，只追踪特定 Agent 的请求：

```bash
agix logs --tail --agent code-reviewer
```

## `agix export`

将用量记录导出为文件，便于后续分析。

```bash
agix export --format csv           # 导出 CSV
agix export --format json          # 导出 JSON
agix export --period 2026-01       # 指定月份（YYYY-MM）
```

| 选项 | 说明 |
|------|------|
| `--format <fmt>` | 输出格式：`csv` / `json` |
| `--period <月份>` | 指定导出月份，格式 `YYYY-MM`（默认当月） |
