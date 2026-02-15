# agix

企业级 LLM 反向代理，内置 Token/成本追踪与共享 MCP 工具。单二进制，零外部依赖。

## 它做什么？

agix 是一个本地 HTTP 网关，位于你的 AI Agent 和 LLM 提供商之间。它透明地转发请求，记录每一次 Token 消耗，计算成本，执行预算控制，并可选地注入共享 MCP 工具 —— 所有这些都不需要修改 Agent 代码。

```
Your Agent → agix proxy (localhost) → OpenAI / Anthropic API
                    ↓                              ↓
              SQLite (用量日志)              响应中有 tool_call?
                    ↓                        ↓            ↓
              CLI 统计 / 日志 / 导出       是            否
                                            ↓             ↓
                                      代理执行工具     返回给 Agent
                                      (通过 MCP)      (累计成本)
                                            ↓
                                      追加结果,
                                      重新发给 LLM
```

## 核心特性

- **Token/成本追踪** —— 每次请求记录输入/输出 Token 数和美元成本
- **预算控制** —— 按 Agent 设置每日/每月预算上限
- **MCP 工具注入** —— Agent 无需修改代码即可使用共享工具
- **多提供商支持** —— 同时支持 OpenAI 和 Anthropic
- **流式传输** —— 完整支持 SSE 流式响应
- **API Key 隔离** —— Agent 永远不会看到真实的 API Key
- **数据本地化** —— 所有数据存储在本地 SQLite 文件中

## 架构

```
┌─────────────────────────────────────────┐
│               CLI (cobra)                │
│  init · start · stats · logs · budget   │
│  export · tools                          │
├─────────────────────────────────────────┤
│            HTTP Proxy Server             │
│  POST /v1/chat/completions → route      │
│  GET  /v1/models           → list       │
│  GET  /health              → ok         │
├──────────┬──────────────────────────────┤
│  Router  │  gpt-* → api.openai.com     │
│          │  claude-* → api.anthropic.com│
├──────────┴──────────────────────────────┤
│  Tool Manager (optional)                 │
│  - MCP clients (stdio JSON-RPC 2.0)     │
│  - Per-agent tool filtering              │
│  - Tool execution loop                   │
├─────────────────────────────────────────┤
│  Intercept: extract usage from response  │
│  Calculate cost via pricing table        │
│  Write record to SQLite                  │
├─────────────────────────────────────────┤
│  SQLite (WAL mode, single file)          │
│  ~/.agix/agix.db                         │
└─────────────────────────────────────────┘
```

## 技术栈

| 组件 | 选择 | 原因 |
|------|------|------|
| 语言 | Go 1.26 | 单二进制，交叉编译，适合网络 I/O |
| CLI | cobra | 行业标准（Docker, K8s, gh CLI 同款） |
| 数据库 | modernc.org/sqlite | 纯 Go，零 CGO |
| 配置 | gopkg.in/yaml.v3 | YAML 配置解析 |
