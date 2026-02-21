# audit · session · webhook

## `agix audit`

查看安全审计日志，记录所有触发防火墙、预算或工具访问控制的事件。

```bash
agix audit list                       # 查看最近审计事件
agix audit list --type tool_call      # 按事件类型筛选
agix audit list --type budget_exceed  # 查看超预算事件
agix audit list --type firewall_block # 查看被防火墙拦截的请求
```

### 常见事件类型

| 类型 | 说明 |
|------|------|
| `tool_call` | Agent 调用了 MCP 工具 |
| `budget_exceed` | Agent 超出每日或每月预算 |
| `firewall_block` | 请求被防火墙规则拦截 |
| `firewall_warn` | 请求触发了防火墙警告规则 |

审计日志由代理在请求处理过程中自动记录，无需额外配置。

## `agix session`

管理会话级配置覆盖。通过 `X-Session-ID` 请求头可为某个会话指定临时配置（如切换模型、调整参数），不影响全局配置。

```bash
agix session list     # 列出所有活跃的会话覆盖配置
agix session clean    # 清理已过期的会话配置
```

### 工作原理

1. Agent 在请求中携带 `X-Session-ID: <id>` 请求头
2. 通过 `POST /v1/sessions/<id>` 为该会话设置覆盖配置（JSON）
3. 代理在处理该会话的后续请求时，使用覆盖配置替换全局配置中的对应字段
4. 会话过期后，`agix session clean` 可清理过期条目

## `agix webhook`

管理出站 Webhook，agix 支持在预算预警、防火墙触发等事件发生时向外部系统发送通知。

```bash
agix webhook list       # 列出已配置的 Webhook
agix webhook history    # 查看 Webhook 调用历史及状态
```

### Webhook 配置（config.yaml）

```yaml
webhooks:
  - name: budget-alert
    url: https://hooks.slack.com/services/xxx
    events: ["budget_alert"]
    secret: "your-hmac-secret"   # 用于 HMAC-SHA256 签名验证
```

### 签名验证

每次 Webhook 调用时，agix 会在请求头中附带 `X-Webhook-Signature: sha256=HEX`，接收方可用共享密钥验证签名，防止伪造。
