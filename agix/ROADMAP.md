# agix Roadmap

Feature ideas inspired by [openclaw/openclaw](https://github.com/openclaw/openclaw), prioritized for incremental implementation.

## 1. Session Memory（向量记忆）

**来源**: openclaw `src/memory/` (78 files)

当前 agix 有 semantic cache（精确/近似匹配单次请求），扩展为跨请求的 agent 长期记忆层。

- Embedding 生成 → SQLite-vec 向量存储
- 混合检索：语义相似 + 关键词匹配
- MMR 去重 + 时间衰减
- 每个 agent 独立记忆空间
- 自动注入相关记忆到 system prompt

**涉及模块**: `internal/memory/`, proxy injection, config

---

## 2. Cron Job 调度

**来源**: openclaw `src/cron/` (50 files)

按 cron 表达式自动向指定 agent 发送预设 prompt。

- cron 表达式解析 + 调度器
- Stagger 防雷群（多个 job 同时触发时错开）
- Catch-up 补执行（服务重启后补发遗漏任务）
- One-shot 单次任务
- 执行日志 + 结果回调

**涉及模块**: `internal/cron/`, `cmd/cron.go`, config

---

## 3. 审计与安全扫描

**来源**: openclaw `src/security/` (24 files)

增强 firewall，加完整审计系统。

- 请求/响应完整内容审计日志（opt-in）
- 敏感 tool 调用审计（哪个 agent 调了什么 tool）
- Tool 危险等级标记 + 调用需审批
- Secret-equal 防时序攻击比较
- 审计日志独立存储，不可篡改

**涉及模块**: `internal/audit/`, firewall 增强, store 扩展

---

## 4. 通用 Webhook 端点

**来源**: openclaw webhook 触发 agent 执行

当前 webhook 仅用于 budget alert，泛化为通用 webhook。

- `POST /v1/webhooks/:name` 触发预配置的 agent 任务
- 请求 payload 可注入到 prompt 模板
- 执行结果异步回调到配置的 URL
- 支持 HMAC 签名验证
- 执行历史可查

**涉及模块**: `internal/webhook/`, proxy routes, config

---

## 5. 会话级配置覆写

**来源**: openclaw session patching (per-session model/thinking level)

同一 agent 的不同请求链可以有不同配置。

- `X-Session-ID` header 标识会话
- Per-session 覆写: model, temperature, prompt template, max_tokens
- Session 配置持久化（SQLite）
- TTL 自动过期清理
- API 端点管理 session 配置

**涉及模块**: `internal/session/`, proxy, config

---

## 6. Agent 间通信

**来源**: openclaw `sessions_send`, `sessions_history`

Agent 输出自动路由为另一个 agent 的输入，实现 pipeline。

- Pipeline 定义: agent-A → agent-B → agent-C
- 中间结果可选持久化
- 条件路由（基于输出内容决定下一步）
- 并行 fan-out / fan-in
- 执行链路追踪

**涉及模块**: `internal/pipeline/`, proxy, config

---

## 7. MCP Tool Bundle

**来源**: openclaw 52 个内置 skill

预定义常用 tool 组合，一键启用。

- Bundle 定义: 名称 + MCP server 列表 + 默认 agent 权限
- 内置 bundle: code-review, devops, docs-writer
- `agix bundle list / install / remove`
- Bundle registry（本地目录或远程）
- 自定义 bundle 打包

**涉及模块**: `internal/bundle/`, `cmd/bundle.go`, config

---

## 8. 请求链路追踪（Input Provenance）

**来源**: openclaw `input-provenance.ts`

每条请求记录完整处理链路。

- 来源 IP / client 标识
- 处理步骤链: firewall → prompt_inject → cache → route → compress → upstream
- 每步耗时 + 是否生效
- Failover 路径记录
- 响应 header 暴露 trace ID (`X-Trace-ID`)
- `agix trace <trace-id>` 查询完整链路

**涉及模块**: `internal/trace/`, proxy 各环节埋点, store

---

## 9. Doctor 健康检查

**来源**: openclaw `openclaw doctor`

一键检查所有配置和依赖的健康状态。

- API key 有效性检查（轻量 models list 请求）
- Budget 配置合理性（日限 < 月限等）
- Firewall 规则语法校验 + 冲突检测
- MCP server 可连接性 + tool 发现
- SQLite 数据库完整性
- 配置文件权限检查（不应 world-readable）
- 输出彩色诊断报告

**涉及模块**: `cmd/doctor.go`, 各模块 health check 接口

---

## 10. 响应策略层

**来源**: openclaw `send-policy.ts`

控制 agent 响应的后处理。

- 长响应自动分块（按 token 或段落）
- 响应格式强制转换（强制 JSON output、强制 markdown）
- 响应内容过滤（移除敏感信息再返回给 agent）
- 响应延迟控制（防刷屏，rate limit 响应端）
- Per-agent 响应策略配置

**涉及模块**: `internal/responsepolicy/`, proxy post-processing, config

---

## 实现优先级建议

| 优先级 | 功能 | 理由 |
|--------|------|------|
| P0 | 9. Doctor 健康检查 | 工作量小，立即提升运维体验 |
| P0 | 8. 请求链路追踪 | 调试利器，对现有代码侵入小 |
| P1 | 3. 审计与安全扫描 | 企业必备，firewall 基础已有 |
| P1 | 5. 会话级配置覆写 | 用户需求明确，架构清晰 |
| P1 | 1. Session Memory | 差异化能力，semantic cache 基础已有 |
| P2 | 4. 通用 Webhook | 与外部系统集成的关键 |
| P2 | 10. 响应策略层 | 与 prompt inject 对称的后处理能力 |
| P2 | 2. Cron Job 调度 | 自动化场景刚需 |
| P3 | 6. Agent 间通信 | 复杂度高，需要更多设计 |
| P3 | 7. MCP Tool Bundle | 生态建设，依赖用户反馈 |
