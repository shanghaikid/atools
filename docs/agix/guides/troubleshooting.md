# 故障排查与常见问题

## 常见问题

### Agent 获得 429 Too Many Requests

**症状**：Agent 的请求被拒绝，返回 429 状态

**可能的原因**：

1. **预算已超出**
   ```bash
   # 检查预算状态
   agix budget

   # 输出显示 Agent 超出限制
   # 解决方案：增加预算或等待重置
   agix budget set agent-name -d 100.0
   ```

2. **频率限制已超出**
   ```yaml
   # 检查配置
   rate_limits:
     expensive-agent:
       requests_per_minute: 10    # Agent 达到此限制

   # 解决方案：增加限制或在 Agent 中实现退避
   ```

3. **防火墙阻止**
   ```bash
   # 检查防火墙阻止
   agix audit list --type firewall_block

   # 解决方案：如果合法请求被阻止，调整防火墙规则
   ```

**快速修复**：按此顺序检查：
```bash
agix budget                        # 1. 预算超出？
agix stats --agent agent-name      # 2. 使用模式？
agix audit list --agent agent-name # 3. 防火墙阻止？
```

---

### 高成本惊人增长

**症状**：每日统计中出现意外的成本峰值

**可能的原因**：

1. **Agent 在无限重试循环中**
   ```bash
   # 检查请求率
   agix logs --tail --agent problematic-agent
   # 注意：几秒内有 100 个请求？

   # 解决方案：重启 Agent，设置频率限制
   agix rate_limits set agent -m 5 --hour 100
   ```

2. **高级模型被错误使用**
   ```bash
   # 检查模型使用
   agix stats --group-by model

   # 看到 gpt-4o 用于简单任务？
   # 解决方案：启用智能路由来自动降级
   ```

3. **缓存禁用（缓存命中率 0%）**
   ```bash
   # 检查缓存命中率
   curl http://localhost:8080/api/stats

   # cache_hit_rate：0？
   # 解决方案：如果适合你的工作负载，启用缓存
   ```

**调试**：追踪成本进展
```bash
# 获取每小时成本趋势
agix stats --group-by day -p 1d    # 今天目前
agix logs -n 500 | head -100       # 最近请求
```

---

### API 密钥错误

**症状**：来自上游提供商的 401/403 未授权

**解决方案**：
```bash
# 1. 验证密钥是否正确
vim ~/.agix/config.yaml
# 仔细检查：keys.openai, keys.anthropic 等

# 2. 运行 doctor 来验证
agix doctor
# 输出将显示哪个密钥无效

# 3. 检查密钥权限
# OpenAI：密钥应具有 "read" 和 "write" 权限
# Anthropic：密钥格式应为 "sk-ant-..."
```

**真实示例**：
```bash
$ agix doctor
...
API 密钥验证
  ✗ OpenAI API 密钥无效（401）
  ✓ Anthropic API 密钥有效
  ✓ DeepSeek API 密钥有效

解决方案：在 config.yaml 中更新 keys.openai
```

---

### 响应缓慢

**症状**：请求耗时 >5 秒才能完成

**诊断**：
```bash
# 1. 检查请求追踪
agix trace list | grep "Duration: [5-9][0-9][0-9][0-9]ms"

# 2. 查看详细追踪
agix trace trace-id-here

# 3. 检查哪个 span 很慢：
#    - api.call > 3000ms → 上游提供商很慢
#    - firewall.scan > 1000ms → 正则表达式模式太复杂
#    - store.write > 500ms → 数据库慢
```

**按组件的解决方案**：

**如果 api.call 很慢（>3s）**：
- 上游提供商很慢
- 检查提供商状态页面
- 考虑故障转移到不同的模型/提供商

**如果 firewall.scan 很慢（>1s）**：
- 自定义正则表达式模式效率低
- 简化模式或对测试使用 "log" 操作
- 禁用未在生产使用的规则

**如果 store.write 很慢（>500ms）**：
- 数据库问题（SQLite 锁定或 PostgreSQL 慢）
- 对于 SQLite：重启 agix 来解锁
- 对于 PostgreSQL：检查索引

---

### 数据库错误

**症状**："database is locked" 或连接超时

**对于 SQLite**：
```bash
# 1. 重启 agix（清除锁定）
pkill agix
sleep 2
agix start

# 2. 检查完整性
agix doctor
# 应显示："✓ 数据库完整性检查通过"

# 3. 如果失败，重建
cp ~/.agix/agix.db ~/.agix/agix.db.backup
rm ~/.agix/agix.db
agix start  # 创建新模式
```

**对于 PostgreSQL**：
```bash
# 1. 检查连接
psql postgres://user:pass@host/agix -c "SELECT 1"

# 2. 检查锁定
psql agix -c "SELECT * FROM pg_locks WHERE NOT granted;"

# 3. 杀死卡住的连接
psql agix -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE usename = 'agix';"

# 4. 重启 agix
pkill agix
agix start
```

---

### MCP 工具不工作

**症状**：Agent 获得 "tool not found" 错误

**诊断**：
```bash
# 1. 列出可用工具
agix tools list

# 2. 检查工具是否被发现
# 如果为空：MCP 服务器没有正确启动

# 3. 检查 MCP 服务器日志
agix doctor | grep "MCP Servers"
# 应显示："✓ filesystem server started (pid: 12345)"

# 如果失败，运行 doctor 获取详情
```

**常见 MCP 问题**：

1. **npm 包未安装**
   ```bash
   npm install -g @modelcontextprotocol/server-filesystem

   # 测试命令
   npx -y @modelcontextprotocol/server-filesystem /tmp
   # 应该响应初始化命令
   ```

2. **工具访问被配置拒绝**
   ```yaml
   # 检查 Agent 工具访问
   tools:
     agents:
       agent-name:
         deny: ["tool_name"]  # 工具在拒绝列表中！

   # 从拒绝列表中移除或添加到允许列表
   ```

3. **工具需要环境变量**
   ```yaml
   tools:
     servers:
       github:
         command: "npx"
         args: ["-y", "@modelcontextprotocol/server-github"]
         env: ["GITHUB_TOKEN=ghp_xxx"]   # 缺失或为空！
   ```

---

### 防火墙阻止合法请求

**症状**：有效请求被拒绝，显示 "firewall blocked"

**解决方案**：
```bash
# 1. 检查哪个规则匹配
agix audit list --type firewall_block -n 5

# 2. 审查规则
vim ~/.agix/config.yaml

# 3. 要么：
#    a) 使规则更具体（更好的正则表达式）
#    b) 将操作从 "block" 改为 "warn"（用于测试）
#    c) 如果误报过多，移除规则

firewall:
  rules:
    - name: "problematic_rule"
      pattern: "..."
      action: "warn"    # 从 "block" 改为
```

**示例：修复过度激进的注入检测**
```yaml
# 太激进：任何地方都阻止 "system"
- name: "injection_old"
  pattern: "system"
  action: "block"

# 更好：仅在一起时阻止 "system prompt"
- name: "injection_new"
  pattern: "(?i)system\\s+prompt"
  action: "block"
```

---

## 常见问题

### Q：我可以将特定 Agent 路由到特定模型吗？

**A**：使用会话覆盖：

```python
# 为 Agent 创建会话
session = requests.post("http://localhost:8080/v1/sessions/agent-session", json={
    "agent_name": "my-agent",
    "model": "gpt-4o-mini"  # 强制此模型
}).json()

# 在请求中使用会话
client = OpenAI(
    base_url="http://localhost:8080/v1",
    extra_headers={"X-Session-ID": session["session_id"]}
)
```

或使用智能路由进行自动成本优化。

---

### Q：我如何降低成本？

**A**：按优先级顺序：

1. **启用智能路由**（最简单，节省 20-30%）
   ```yaml
   routing:
     enabled: true
     # 简单请求 → 更便宜的模型
   ```

2. **启用语义缓存**（如果适用，节省 20-40%）
   ```yaml
   cache:
     enabled: true
   ```

3. **切换到更便宜的模型**（节省 30-50%）
   - gpt-4o-mini 而不是 gpt-4o
   - claude-haiku 而不是 claude-opus
   - deepseek-chat 用于一般任务

4. **设置频率限制**（防止失控成本）
   ```yaml
   rate_limits:
     agent: {requests_per_minute: 10}
   ```

---

### Q：我可以使用多个 LLM 提供商吗？

**A**：是的！将模型路由到不同的提供商：

```yaml
keys:
  openai: "sk-..."
  anthropic: "sk-ant-..."
  deepseek: "sk-..."

# 请求自动路由：
# - gpt-4o → OpenAI
# - claude-opus-4-6 → Anthropic
# - deepseek-chat → DeepSeek
```

使用故障转移进行提供商出错时的自动切换。

---

### Q：我如何导出数据用于报告？

**A**：
```bash
# CSV 导出（Excel 友好）
agix export --format csv -o costs.csv

# JSON 导出（用于分析）
agix export --format json -o costs.json

# 特定时期
agix export --format csv --period 2026-01 -o january.csv

# 在电子表格中打开
open costs.csv
```

---

### Q：我可以在自定义时间重置预算吗？

**A**：目前预算在 UTC 午夜重置。变通方案：

```python
# 创建自定义模型会话用于部分一天
import os
from datetime import datetime, timedelta

# 如果 UTC 之前的中午，使用高级模型
# 如果 UTC 之后的中午，使用廉价模型
hour = datetime.utcnow().hour
model = "gpt-4o" if hour < 12 else "gpt-4o-mini"

# 创建会话，TTL 到重置
session = requests.post("http://localhost:8080/v1/sessions/my-session", json={
    "agent_name": "my-agent",
    "model": model,
    "ttl": f"{24-hour}h"
}).json()
```

---

### Q：我如何在生产中监控 agix？

**A**：设置：

1. **启用所有可观测性**
   ```yaml
   tracing:
     enabled: true
     sample_rate: 0.5

   audit:
     enabled: true

   dashboard:
     enabled: true
   ```

2. **每天检查**
   ```bash
   agix stats --period 1d       # 昨天的成本
   agix logs -n 100              # 最近请求
   agix audit list -n 50         # 安全事件
   ```

3. **设置告警**
   ```bash
   # 通过检查预算状态的 cron 作业
   agix budget | grep "%" | grep -E "[8-9][0-9]%|100%"
   # 如果匹配，发送告警
   ```

4. **每周审查**
   ```bash
   agix stats --group-by agent  # 按 Agent 分析
   agix export --format csv     # 用于报告
   ```

---

### Q：最大请求速率是多少？

**A**：取决于你的硬件和数据库：

- **SQLite**：100-500 请求/秒（单服务器）
- **PostgreSQL**：1000+ 请求/秒（使用适当索引）

要测试：
```bash
# 监控请求吞吐量
agix logs --tail

# 检查数据库性能
agix doctor
```

对于大量使用，使用带连接池的 PostgreSQL。

---

### Q：我可以备份我的数据吗？

**A**：

**SQLite**：
```bash
# 简单文件复制
cp ~/.agix/agix.db /backup/agix.db

# 或 SQL 导出
sqlite3 ~/.agix/agix.db ".dump" > backup.sql
```

**PostgreSQL**：
```bash
# 备份
pg_dump agix > backup.sql

# 恢复
psql agix < backup.sql
```

---

### Q：我如何排查质量门控问题？

**A**：
```bash
# 检查日志中的质量警告
agix logs -n 100 | grep -i "quality"

# 检查追踪中的失败响应
agix trace list | head -20

# 查找 on_empty/on_truncated/on_refusal 触发器
# 审查 response_policy 查看是否发生截断
```

---

## 支持

如需更多帮助：

- **GitHub Issues**：报告 bug 或请求功能
- **文档**：查看 `/docs/agix/` 获取详细指南
- **健康检查**：运行 `agix doctor` 诊断问题
- **审计追踪**：查看 `agix audit list` 查看安全事件
- **日志**：检查 `agix logs --tail` 查看实时活动
