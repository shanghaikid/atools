# doctor

## `agix doctor`

运行全套健康检查，验证配置文件、API 密钥、预算规则、防火墙配置和数据库是否就绪。

```bash
agix doctor
```

### 输出格式

每项检查结果以彩色标识开头（`PASS` / `WARN` / `FAIL`）：

```
  agix doctor

  PASS  Config file: /Users/you/.agix/config.yaml permissions OK (600)
  PASS  API keys: 2/2 valid
         openai: valid
         anthropic: valid
  PASS  Budgets: 3 agent(s) configured OK
  PASS  Firewall: 2 rule(s) valid
  WARN  Database: /Users/you/.agix/agix.db does not exist (will be created on first start)

  All checks passed!
```

若有检查失败，退出码等于失败项数量（`> 0`）；全部通过则退出码为 `0`，便于在 CI 中使用。

### 检查项说明

| 检查项 | 说明 | PASS | WARN | FAIL |
|--------|------|------|------|------|
| **Config file permissions** | 验证配置文件权限是否为 `0600`（含 API 密钥，不应被其他用户读取） | 权限为 `0600` | 权限过宽（组或其他用户可读） | 无法读取文件元信息 |
| **API key validity** | 向各 provider 发起轻量请求（`GET /models`）验证密钥有效性；OpenAI/DeepSeek 使用 `Authorization: Bearer`，Anthropic 使用 `x-api-key` | 所有已配置密钥有效 | 未配置任何 provider | 存在无效密钥（HTTP 401/403） |
| **Budget configuration** | 验证预算规则逻辑合理性：`daily ≤ monthly`，`alert_at_percent` 在 `[1, 100]` 范围内 | 所有规则合法 | 存在不合理规则 | — |
| **Firewall rules** | 编译每条自定义正则，验证 `action` 字段为 `block` / `warn` / `log` 之一 | 全部规则合法 | — | 存在非法正则或未知 action |
| **Database connectivity** | SQLite：检查文件存在性并运行 `PRAGMA integrity_check`；PostgreSQL：Ping + `SELECT version()` | 数据库健康 | SQLite 文件尚不存在（首次启动时自动创建） | 连接失败或完整性异常 |

### 常见问题

**WARN: config permissions**

```bash
chmod 600 ~/.agix/config.yaml
```

**FAIL: API keys — invalid key (HTTP 401)**

配置文件中对应 provider 的密钥无效或已过期，更新后重新运行 `agix doctor`。

**WARN: database does not exist**

正常情况——数据库文件在 `agix start` 首次启动时自动创建，无需手动处理。

**FAIL: database integrity check**

数据库文件损坏，建议备份后删除 `~/.agix/agix.db` 让代理重新初始化。
