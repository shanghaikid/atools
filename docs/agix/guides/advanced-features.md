# 高级功能

## 概述

agix 包含适用于专门用例的多项高级能力：

- **系统提示词注入** — 注入全局或按 Agent 的系统提示词
- **MCP 工具包** — 适用于常见工作流的预打包工具集
- **PostgreSQL 后端** — 可扩展的大规模部署替代方案
- **DeepSeek 提供商支持** — 与 OpenAI/Anthropic 并列的额外 LLM 提供商

## 系统提示词注入

系统提示词注入允许你向每个请求的系统提示词前置或后置文本。

### 工作原理

1. 请求从 Agent 到达
2. 如果启用了提示词注入：
   - 选择适用的提示词（全局或按 Agent）
   - 前置或后置到现有系统提示词
3. 转发修改后的请求到 LLM
4. LLM 使用注入的提示词处理

### 配置

```yaml
prompt_templates:
  enabled: true

  # 适用于所有 Agent 的全局模板
  global: "你是一个有帮助的助手。遵循公司政策。"

  # 按 Agent 的覆盖
  agents:
    code-reviewer: "你是专家代码审查官。关注安全性和性能。"
    docs-writer: "你是技术文档编辑。使用清晰、简洁的语言。"
    compliance-checker: "你是合规官。根据政策检查所有响应。"

  # 位置：prepend（在用户提示词前）或 append（在后）
  position: "prepend"              # 默认：prepend
```

### 真实示例：执行公司政策

```yaml
prompt_templates:
  enabled: true

  global: |
    你是公司 AI 助手。

    重要：
    - 从不推荐竞争对手产品
    - 总是首先提及公司产品
    - 遵守数据隐私法规
    - 不参与政治讨论

  agents:
    customer-support:
      template: |
        你是友好的客户支持 Agent。
        优先考虑客户满意度。
        提供最佳方案，而不是最有利可图的方案。
      position: "prepend"

    sales-agent:
      template: |
        你是销售助手。
        推荐适合客户需求的公司产品。
        突显竞争优势。
```

### 对成本的影响

注入的提示词增加 Token 用量：

```
不注入：
- 用户："Hello" （10 tokens）
- LLM 处理：10 tokens

注入后：
- 系统："你是有帮助的助手..." （20 tokens）
- 用户："Hello" （10 tokens）
- LLM 处理：30 tokens
- 成本增加：系统提示词 Token 数的 2 倍
```

### 位置：Prepend vs Append

**Prepend**（默认）：
- 系统提示词优先
- 在 LLM 推理中有优先级
- 更可靠

**Append**：
- 系统提示词在用户消息后
- 用户消息有优先级
- 适合软性指南

```yaml
# 示例：软性指南 (append)
agents:
  flexible-agent:
    template: "尽量帮助，不要太无聊。"
    position: "append"
```

## MCP 工具包

工具包是适用于常见工作流的预打包 MCP 服务器集合。

### 内置包

agix 包含几个预配置的包：

```bash
# 列出可用的包
agix bundle list

# 输出：
# 名称          描述
# basic         文件操作（读、写、浏览）
# github        GitHub 仓库访问（读、搜索、贡献）
# code-review   代码分析和审查工具
# devops        基础设施和部署工具
# docs-writer   文档和发布工具
```

### 安装包

```bash
# 安装包
agix bundle install basic

# 显示包详情
agix bundle show github
# 输出：
# 包：github
# 描述：GitHub 仓库访问
# 服务器：
#   - github：GitHub 客户端（需要 GITHUB_TOKEN 环境变量）
```

### 创建自定义包

在配置中定义包：

```yaml
bundles: ["basic", "github", "custom"]

tools:
  servers:
    # 包：basic（隐式，来自内置）

    # 包：github（隐式，来自内置）

    # 自定义工具（不在包中）
    custom-api:
      command: "npx"
      args: ["-y", "@company/custom-api-server"]
      env: ["API_KEY=..."]

    internal-tools:
      command: "/usr/local/bin/internal-tools-server"
```

然后在配置中引用：

```yaml
bundles:
  - basic           # 使用内置包
  - github          # 使用内置包
  - code-review     # 如果可用，使用内置包
```

### 工具访问控制

控制哪些 Agent 可以使用哪些工具：

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
    # 代码审查官：只读文件访问
    code-reviewer:
      allow: ["read_file", "list_directory"]

    # 文档编辑：完整文件系统 + github
    docs-writer:
      allow: ["*"]                 # 允许所有工具

    # 安全 Agent：仅限沙箱执行
    security-agent:
      deny: ["delete_file", "modify_permissions"]

    # 其他人：默认无工具
```

### 真实示例：文档工作流

```bash
# 安装文档包
agix bundle install docs-writer

# 配置：
bundles:
  - docs-writer              # 包含文件系统 + github + 发布工具

tools:
  agents:
    docs-agent:
      allow: ["read_file", "write_file", "list_directory", "git_commit"]
```

现在 `docs-agent` 可以：
- 读取代码文件以获取示例
- 编写文档
- 提交更改到 git

全部无需修改 Agent 代码！

## PostgreSQL 后端

对于具有许多请求的大规模部署，PostgreSQL 提供比 SQLite 更好的可扩展性。

### 设置

1. 创建 PostgreSQL 数据库：

```bash
createdb agix
psql agix -c "CREATE ROLE agix WITH PASSWORD 'password' CREATEDB;"
ALTER ROLE agix CREATEDB;
```

2. 配置 agix：

```yaml
database: "postgres://agix:password@localhost:5432/agix?sslmode=disable"
```

3. 运行 agix（模式自动创建）：

```bash
agix start
```

### 配置变化

**生产环境（使用 SSL）：**
```yaml
database: "postgres://agix:password@prod-db.example.com:5432/agix?sslmode=require"
```

**使用 SSL 证书：**
```yaml
database: "postgres://agix:password@prod-db.example.com:5432/agix?sslmode=verify-full&sslrootcert=/path/to/ca.pem"
```

**通过 pgBouncer 的连接池：**
```yaml
database: "postgres://agix:password@pgbouncer.example.com:6432/agix?sslmode=disable"
```

### SQLite vs PostgreSQL

| 功能 | SQLite | PostgreSQL |
|------|--------|------------|
| 设置 | 自动（文件） | 需要服务器 |
| 查询 | 单机器 | 分布式 |
| 并发 | 良好（WAL 模式） | 优秀 |
| 磁盘空间 | MB-GB | 无限 |
| 备份 | 文件复制 | `pg_dump` |
| 复制 | 无 | 是（流复制） |

**使用 SQLite 时：**
- 单服务器
- <1M 请求/天
- 不需要分布式备份

**使用 PostgreSQL 时：**
- 多个服务器
- >1M 请求/天
- 需要高可用性
- 需要备份/复制

### 迁移：SQLite → PostgreSQL

```bash
# 1. 导出 SQLite 数据
agix export --format json > backup.json

# 2. 更新配置为使用 PostgreSQL
vim ~/.agix/config.yaml
# database: "postgres://..."

# 3. 重启 agix（创建模式）
agix start

# 4. 导入数据（手动，取决于你的脚本）
# 使用 JSON 备份来填充 PostgreSQL
```

### 性能调优

对于具有高请求量的 PostgreSQL：

```sql
-- 为常见查询创建索引
CREATE INDEX idx_requests_agent_timestamp
  ON requests (agent_name, timestamp DESC);

CREATE INDEX idx_requests_model
  ON requests (model, timestamp DESC);

-- 分析以进行查询优化
ANALYZE requests;
```

## DeepSeek 提供商支持

agix 支持 DeepSeek 模型与 OpenAI 和 Anthropic 并列。

### 配置

```yaml
keys:
  openai: "sk-..."
  anthropic: "sk-ant-..."
  deepseek: "sk-..."

# DeepSeek 模型自动路由
# 仅需使用模型名称如 "deepseek-chat"
```

### 支持的模型

```
deepseek-chat        # 通用聊天
deepseek-reasoner    # 推理/分析
```

### 路由

模型名称 → 提供商映射：

```
gpt-* → OpenAI
claude-* → Anthropic
deepseek-* → DeepSeek
```

### 成本示例

比较提供商：

```
任务：总结 2000 字

OpenAI (gpt-4o-mini)：
- 输入：500 tokens × $0.00015 = $0.075
- 输出：200 tokens × $0.0006 = $0.00012
- 总计：$0.07512

Anthropic (claude-haiku)：
- 输入：500 tokens × $0.00008 = $0.04
- 输出：200 tokens × $0.00024 = $0.000048
- 总计：$0.040048

DeepSeek (deepseek-chat)：
- 输入：500 tokens × $0.00014 = $0.07
- 输出：200 tokens × $0.00028 = $0.000056
- 总计：$0.070056

最佳选项：Anthropic Haiku（比 GPT-4o-mini 便宜 55%）
```

### 故障转移模式：跨提供商

```yaml
failover:
  chains:
    gpt-4o:
      - "gpt-4o-mini"         # 更便宜的 OpenAI
      - "deepseek-chat"       # 跨提供商
      - "claude-opus-4-6"     # 最后的手段

    claude-opus-4-6:
      - "claude-sonnet-4-5-20250929"  # 更便宜的 Anthropic
      - "deepseek-chat"              # 跨提供商
      - "gpt-4o"                     # 最后的手段
```

## 组合高级功能

### 真实示例：企业 SaaS 设置

```yaml
# 为扩展性使用 PostgreSQL
database: "postgres://agix:password@prod-db.example.com:5432/agix?sslmode=require"

# 为安全注入系统提示词
prompt_templates:
  enabled: true
  global: |
    你是企业 AI 助手。
    遵守 GDPR 和公司安全政策。
  agents:
    customer-support:
      template: "总是友好和专业。"

# 为生产力使用工具包
bundles:
  - basic
  - github
  - code-review

tools:
  agents:
    developer-bot:
      allow: ["*"]                # 为可信 Bot 完整访问

    customer-support:
      deny: ["delete_file", "modify_permissions", "git_commit"]  # 只读

# 跨提供商故障转移
failover:
  chains:
    gpt-4o:
      - "gpt-4o-mini"
      - "claude-opus-4-6"
      - "deepseek-chat"

# 频率限制 + 预算
rate_limits:
  api-consumer:
    requests_per_minute: 100
    requests_per_hour: 5000

budgets:
  api-consumer:
    daily_limit_usd: 50.0
    monthly_limit_usd: 1000.0
    alert_at_percent: 80
```

## 最佳实践

1. **谨慎使用系统提示词** — 仅用于关键政策，不用于一般性指导
2. **仅安装所需的包** — 减少复杂性和工具表面积
3. **早期迁移到 PostgreSQL** — 最好在高流量前计划
4. **跨提供商故障转移** — DeepSeek 作为廉价回退，Anthropic 获得质量
5. **测试提供商切换** — 提供商之间的质量可能有所不同
6. **监控包更新** — 内置包可能会获得新的/更改的工具
