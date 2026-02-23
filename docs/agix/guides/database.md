# 数据库操作指南

agix 支持 SQLite 和 PostgreSQL 两种数据库后端，本指南覆盖数据库模型、迁移策略、备份恢复和常用查询。

## 数据模型

agix 使用单个 `requests` 表记录所有 API 请求的消耗数据：

```sql
CREATE TABLE IF NOT EXISTS requests (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp     DATETIME NOT NULL DEFAULT (datetime('now')),
    agent_name    TEXT NOT NULL DEFAULT '',
    model         TEXT NOT NULL,
    provider      TEXT NOT NULL,
    input_tokens  INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cost_usd      REAL NOT NULL DEFAULT 0,
    duration_ms   INTEGER NOT NULL DEFAULT 0,
    status_code   INTEGER NOT NULL DEFAULT 200
);
```

**字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | INTEGER | 主键，自增 |
| `timestamp` | DATETIME | 请求时间，ISO 8601 格式（`2026-02-23T12:34:56Z`） |
| `agent_name` | TEXT | 发起请求的 Agent 名称 |
| `model` | TEXT | 使用的模型（如 `gpt-4-turbo`、`claude-3-sonnet`） |
| `provider` | TEXT | 模型提供商（`openai`、`anthropic` 等） |
| `input_tokens` | INTEGER | 输入 token 数 |
| `output_tokens` | INTEGER | 输出 token 数 |
| `cost_usd` | REAL | 本次请求成本（美元） |
| `duration_ms` | INTEGER | 请求耗时（毫秒） |
| `status_code` | INTEGER | HTTP 状态码 |

**索引**：默认在 `timestamp`、`agent_name`、`model` 字段建立索引，支持快速查询。

## SQLite vs PostgreSQL

| 特性 | SQLite | PostgreSQL |
|------|--------|-----------|
| 配置 | 零配置，自动创建文件 | 需要数据库服务 |
| 并发 | 良好（WAL 模式） | 优秀 |
| 多实例共享 | ❌ | ✅ |
| 存储上限 | GB 级 | 无限制 |
| 备份 | 文件复制 | `pg_dump` |
| 流式复制 | ❌ | ✅ |
| 适用场景 | 单机 / < 100 万请求/天 | 多机 / 高并发 / 高可用 |

## SQLite 快速开始

### 默认位置

```bash
~/.agix/agix.db
```

### 配置

在 `config.yaml` 中指定：

```yaml
database: "~/.agix/agix.db"
```

或绝对路径：

```yaml
database: "/var/lib/agix/agix.db"
```

### 管理

```bash
# 查看数据库文件大小
ls -lh ~/.agix/agix.db

# 手动备份
cp ~/.agix/agix.db ~/.agix/agix.db.backup

# 使用 agix 内置导出
agix export --format json > requests.json
```

## SQLite → PostgreSQL 迁移

### 前置条件

1. PostgreSQL 服务已部署且可访问
2. 已创建数据库和用户
3. agix 配置文件可编辑

### 迁移步骤

#### 1. 创建 PostgreSQL 数据库

```bash
# 连接到 PostgreSQL（默认用户 postgres）
psql -U postgres

# 创建数据库
CREATE DATABASE agix;

# 创建用户（建议使用强密码）
CREATE USER agix WITH PASSWORD 'your-secure-password';

# 授予权限
GRANT ALL PRIVILEGES ON DATABASE agix TO agix;

# 退出 psql
\q
```

#### 2. 验证连接

```bash
# 使用 agix 用户连接测试
psql -U agix -d agix -h localhost -c "SELECT 1;"
```

#### 3. 导出现有数据（可选）

如果想保留 SQLite 中的历史数据：

```bash
# 导出 JSON 格式（包含所有字段）
agix export --format json > sqlite_backup.json

# 导出 CSV 格式
agix export --format csv > sqlite_backup.csv
```

**注意**：`agix export` 仅导出 `requests` 表，其他数据表（Session、审计日志等）需要单独处理。

#### 4. 更新配置

编辑 `~/.agix/config.yaml`，替换 `database` 字段：

```yaml
# 修改前
database: "~/.agix/agix.db"

# 修改后（开发环境，无 SSL）
database: "postgres://agix:your-secure-password@localhost:5432/agix?sslmode=disable"

# 生产环境建议配置
database: "postgres://agix:your-secure-password@prod-db.example.com:5432/agix?sslmode=require"
```

#### 5. 启动 agix

```bash
agix start
```

agix 会自动：
- 检测 PostgreSQL 连接字符串
- 创建所有必要的表和索引
- 无需手动执行 DDL 语句

#### 6. 验证迁移

```bash
# 检查新数据是否正常记录
agix logs --limit 10

# 查看成本统计
agix stats
```

#### 7. 导入历史数据（可选）

如果导出了 SQLite 数据，可编写脚本导入到 PostgreSQL：

```bash
# JSON 格式导入示例（伪代码）
cat sqlite_backup.json | jq -r '.[] | [.timestamp, .agent_name, .model, .provider, .input_tokens, .output_tokens, .cost_usd, .duration_ms, .status_code] | @csv' > import.csv

# 使用 psql COPY 命令导入
psql -U agix -d agix -c "COPY requests(timestamp, agent_name, model, provider, input_tokens, output_tokens, cost_usd, duration_ms, status_code) FROM STDIN WITH CSV;" < import.csv
```

### 迁移后清理

完全确认数据已迁移成功后，可删除 SQLite 文件：

```bash
# 备份原文件（保险起见）
mv ~/.agix/agix.db ~/.agix/agix.db.migrated

# 确认无误后可删除
rm ~/.agix/agix.db.migrated
```

## PostgreSQL 连接字符串

### 标准格式

```
postgres://[user[:password]@][netloc][:port][/dbname][?param1=value1&...]
```

### 常见配置

#### 开发环境（本地，无认证）

```yaml
database: "postgres://agix@localhost:5432/agix?sslmode=disable"
```

#### 开发环境（本地，密码认证）

```yaml
database: "postgres://agix:password@localhost:5432/agix?sslmode=disable"
```

#### 生产环境（SSL 必需）

```yaml
database: "postgres://agix:password@prod-db.example.com:5432/agix?sslmode=require"
```

#### 生产环境（SSL + 证书验证）

```yaml
database: "postgres://agix:password@prod-db.example.com:5432/agix?sslmode=verify-full&sslrootcert=/path/to/ca.pem"
```

#### 通过连接池（pgBouncer）

```yaml
database: "postgres://agix:password@pgbouncer.example.com:6432/agix?sslmode=disable"
```

### SSL 模式说明

| 模式 | 说明 | 适用场景 |
|------|------|---------|
| `disable` | 不使用 SSL | 开发环境或内网 |
| `allow` | 如果服务器支持则使用 | 兼容模式 |
| `prefer` | 优先使用 SSL，服务器不支持则降级 | 安全优先但允许降级 |
| `require` | 强制使用 SSL | 生产环境 |
| `verify-ca` | SSL + 验证服务器证书 | 高安全需求 |
| `verify-full` | SSL + 验证服务器证书 + 验证主机名 | 最高安全需求 |

## 多实例 PostgreSQL 配置

多个 agix 实例共享同一 PostgreSQL 数据库可实现负载均衡和高可用：

```
┌──────────────┐
│ agix 实例 1  │
├──────────────┤  ┌──────────────────┐
│ agix 实例 2  │──┤   PostgreSQL     │ ← 统一存储
├──────────────┤  │  （主从复制）    │
│ agix 实例 3  │  └──────────────────┘
└──────────────┘
```

### 配置要点

1. **相同连接字符串**：所有实例使用相同的 PostgreSQL 连接配置

   ```yaml
   # 所有实例的 config.yaml
   database: "postgres://agix:password@prod-db.example.com:5432/agix?sslmode=require"
   ```

2. **并发安全**：agix 所有写操作均为异步插入，数据库驱动（`lib/pq`）保证并发安全

3. **负载均衡**：配合负载均衡器（如 nginx、HAProxy）分发请求

   ```
   Client → Load Balancer → agix 实例 1
                         → agix 实例 2
                         → agix 实例 3
   ```

4. **健康检查**：

   ```bash
   # 检查 agix 健康状态
   curl -s http://localhost:7777/health | jq .

   # 检查数据库连接
   agix doctor
   ```

### 多实例部署示例

#### Docker Compose

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: agix
      POSTGRES_USER: agix
      POSTGRES_PASSWORD: your-secure-password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  agix1:
    image: agix:latest
    environment:
      DATABASE: "postgres://agix:your-secure-password@postgres:5432/agix?sslmode=disable"
    ports:
      - "7777:7777"
    depends_on:
      - postgres

  agix2:
    image: agix:latest
    environment:
      DATABASE: "postgres://agix:your-secure-password@postgres:5432/agix?sslmode=disable"
    ports:
      - "7778:7777"
    depends_on:
      - postgres

  nginx:
    image: nginx:alpine
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
    ports:
      - "8080:8080"
    depends_on:
      - agix1
      - agix2

volumes:
  postgres_data:
```

#### Kubernetes 部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agix
spec:
  replicas: 3
  selector:
    matchLabels:
      app: agix
  template:
    metadata:
      labels:
        app: agix
    spec:
      containers:
      - name: agix
        image: agix:latest
        ports:
        - containerPort: 7777
        env:
        - name: DATABASE
          valueFrom:
            secretKeyRef:
              name: agix-secrets
              key: database-url
        livenessProbe:
          httpGet:
            path: /health
            port: 7777
          initialDelaySeconds: 10
          periodSeconds: 10
```

## 备份与恢复

### 全量备份

#### SQLite

```bash
# 简单文件复制
cp ~/.agix/agix.db ~/.agix/agix.db.$(date +%Y%m%d_%H%M%S)

# 或使用 agix 内置导出（推荐）
agix export --format json > agix_backup_$(date +%Y%m%d).json
```

#### PostgreSQL

```bash
# 使用 pg_dump（纯 SQL 格式）
pg_dump -U agix -d agix -h localhost -f agix_backup.sql

# 使用 pg_dump（二进制格式，更紧凑）
pg_dump -U agix -d agix -h localhost -Fc -f agix_backup.dump

# 压缩备份
pg_dump -U agix -d agix -h localhost | gzip > agix_backup_$(date +%Y%m%d).sql.gz

# 指定备份文件名
pg_dump -U agix -d agix -h localhost \
  --exclude-table-data=audit_logs \
  -f agix_backup_no_audit_$(date +%Y%m%d).sql
```

### 增量备份

#### PostgreSQL WAL 存档

```bash
# 配置 PostgreSQL 启用 WAL 存档
# postgresql.conf
wal_level = replica
archive_mode = on
archive_command = 'cp %p /backup/wal_archive/%f'
archive_timeout = 300
```

### 恢复

#### SQLite

```bash
# 恢复文件
cp ~/.agix/agix.db.backup ~/.agix/agix.db

# 或从 JSON 导入（自定义脚本）
```

#### PostgreSQL

```bash
# 从 SQL 文件恢复（确保数据库为空）
psql -U agix -d agix -h localhost < agix_backup.sql

# 从压缩文件恢复
gunzip -c agix_backup.sql.gz | psql -U agix -d agix -h localhost

# 从二进制转储恢复
pg_restore -U agix -d agix -h localhost agix_backup.dump

# 恢复到新数据库（创建新库后恢复）
createdb agix_restored
psql -U agix -d agix_restored < agix_backup.sql
```

### 恢复验证

```bash
# 检查表是否存在
agix logs --limit 1

# 查看记录数
agix stats

# 详细验证
psql -U agix -d agix -c "SELECT COUNT(*) as request_count, MIN(timestamp) as earliest, MAX(timestamp) as latest FROM requests;"
```

### 备份策略建议

| 备份类型 | 频率 | 保留期 | 用途 |
|---------|------|--------|------|
| 全量备份 | 每天 1 次 | 7 天 | 日常恢复 |
| 周备份 | 每周 1 次 | 4 周 | 更久远的恢复 |
| 月备份 | 每月 1 次 | 12 个月 | 长期存档 |
| WAL 存档 | 持续 | 按需 | 时间点恢复（PITR） |

```bash
# 自动备份脚本示例
#!/bin/bash
BACKUP_DIR="/backup/agix"
DATE=$(date +%Y%m%d_%H%M%S)

# 每天全量备份
pg_dump -U agix -d agix | gzip > $BACKUP_DIR/agix_full_$DATE.sql.gz

# 保留 7 天内的备份
find $BACKUP_DIR -name "agix_full_*.sql.gz" -mtime +7 -delete
```

## 常用查询示例

### 基本查询

#### 查看所有记录

```sql
SELECT * FROM requests ORDER BY timestamp DESC LIMIT 100;
```

#### 统计总记录数

```sql
SELECT COUNT(*) as total_requests FROM requests;
```

#### 查看日期范围

```sql
SELECT
    MIN(timestamp) as earliest,
    MAX(timestamp) as latest,
    COUNT(*) as total
FROM requests;
```

### 成本分析

#### 按 Agent 统计成本

```sql
SELECT
    agent_name,
    COUNT(*) as request_count,
    SUM(input_tokens) as total_input,
    SUM(output_tokens) as total_output,
    ROUND(SUM(cost_usd), 4) as total_cost,
    ROUND(AVG(cost_usd), 6) as avg_cost
FROM requests
GROUP BY agent_name
ORDER BY total_cost DESC;
```

#### 按模型统计成本

```sql
SELECT
    model,
    provider,
    COUNT(*) as request_count,
    SUM(input_tokens) as total_input,
    SUM(output_tokens) as total_output,
    ROUND(SUM(cost_usd), 4) as total_cost
FROM requests
GROUP BY model, provider
ORDER BY total_cost DESC;
```

#### 按提供商统计

```sql
SELECT
    provider,
    COUNT(*) as request_count,
    ROUND(SUM(cost_usd), 4) as total_cost,
    ROUND(AVG(duration_ms), 0) as avg_duration_ms
FROM requests
GROUP BY provider;
```

### 时间分析

#### 按小时分析成本趋势

```sql
-- SQLite
SELECT
    strftime('%Y-%m-%d %H:00:00', timestamp) as hour,
    COUNT(*) as request_count,
    ROUND(SUM(cost_usd), 4) as hourly_cost
FROM requests
GROUP BY strftime('%Y-%m-%d %H:00:00', timestamp)
ORDER BY hour DESC;

-- PostgreSQL
SELECT
    date_trunc('hour', timestamp::timestamp) as hour,
    COUNT(*) as request_count,
    ROUND(SUM(cost_usd), 4) as hourly_cost
FROM requests
GROUP BY date_trunc('hour', timestamp::timestamp)
ORDER BY hour DESC;
```

#### 按天分析成本趋势

```sql
-- SQLite
SELECT
    DATE(timestamp) as date,
    COUNT(*) as request_count,
    ROUND(SUM(cost_usd), 4) as daily_cost
FROM requests
GROUP BY DATE(timestamp)
ORDER BY date DESC
LIMIT 30;

-- PostgreSQL
SELECT
    DATE(timestamp) as date,
    COUNT(*) as request_count,
    ROUND(SUM(cost_usd), 4) as daily_cost
FROM requests
GROUP BY DATE(timestamp)
ORDER BY date DESC
LIMIT 30;
```

### 性能分析

#### 最慢的请求

```sql
SELECT
    timestamp,
    agent_name,
    model,
    duration_ms,
    input_tokens,
    output_tokens,
    cost_usd
FROM requests
ORDER BY duration_ms DESC
LIMIT 20;
```

#### Token 效率分析

```sql
SELECT
    agent_name,
    model,
    AVG(CAST(output_tokens AS FLOAT) / NULLIF(input_tokens, 0)) as token_ratio,
    AVG(cost_usd / NULLIF(input_tokens + output_tokens, 0)) as cost_per_token,
    COUNT(*) as request_count
FROM requests
WHERE input_tokens > 0
GROUP BY agent_name, model
ORDER BY cost_per_token DESC;
```

### 状态和错误分析

#### 按状态码统计

```sql
SELECT
    status_code,
    COUNT(*) as count,
    ROUND(100.0 * COUNT(*) / (SELECT COUNT(*) FROM requests), 2) as percentage
FROM requests
GROUP BY status_code
ORDER BY count DESC;
```

#### 异常状态请求

```sql
SELECT
    timestamp,
    agent_name,
    model,
    status_code,
    duration_ms
FROM requests
WHERE status_code != 200
ORDER BY timestamp DESC
LIMIT 50;
```

### 索引优化查询

#### 查看现有索引

```sql
-- SQLite
SELECT name, sql FROM sqlite_master WHERE type='index' AND tbl_name='requests';

-- PostgreSQL
SELECT indexname, indexdef FROM pg_indexes WHERE tablename='requests';
```

#### 常用索引建议

```sql
-- 按 Agent + 时间查询（stats 命令常用）
CREATE INDEX idx_requests_agent_timestamp ON requests (agent_name, timestamp DESC);

-- 按模型查询
CREATE INDEX idx_requests_model ON requests (model, timestamp DESC);

-- 按提供商查询
CREATE INDEX idx_requests_provider ON requests (provider, timestamp DESC);

-- 时间范围查询
CREATE INDEX idx_requests_timestamp ON requests (timestamp DESC);

-- 成本分析查询
CREATE INDEX idx_requests_cost ON requests (agent_name, cost_usd, timestamp DESC);
```

#### 更新统计信息（PostgreSQL）

```sql
-- 定期执行以优化查询
ANALYZE requests;

-- 或使用脚本定期执行
ANALYZE;
```

## 常见问题

### Q: SQLite 和 PostgreSQL 如何自动切换？

**A**: agix 根据 `database` 字段的值自动判断：

| 前缀 | 驱动 |
|------|------|
| `postgres://` 或 `postgresql://` | PostgreSQL |
| 其他（文件路径） | SQLite |

无需任何额外配置，修改连接字符串即自动切换。

### Q: 能否在运行时切换数据库？

**A**: 不能。需要：
1. 停止 agix
2. 修改 `config.yaml` 的 `database` 字段
3. 重启 agix

为避免数据丢失，迁移前建议先导出数据。

### Q: PostgreSQL 连接失败怎么办？

**A**: 使用 `agix doctor` 诊断：

```bash
agix doctor
```

检查项包括：
- 配置文件格式
- 数据库连接
- 表和索引
- 磁盘空间

### Q: 如何处理大量数据清理？

**A**: 对于超过保留期的旧数据，可归档或删除：

```sql
-- 删除 90 天前的数据（谨慎操作）
DELETE FROM requests
WHERE timestamp < datetime('now', '-90 days');

-- 或导出后删除
agix export --format json --period 2025-11 > old_data.json
DELETE FROM requests WHERE timestamp < '2025-12-01';
```

### Q: 多实例部署时如何防止数据重复？

**A**: agix 记录请求到 PostgreSQL 是原子操作，数据库驱动保证：
- 并发安全
- 无重复主键
- 无数据丢失

无需额外配置即保证一致性。

### Q: 备份频率应该多少？

**A**: 根据业务需求和数据量：
- 开发环境：每周 1 次
- 生产环境（低流量）：每天 1 次
- 生产环境（高流量）：每 6 小时 1 次 + WAL 存档

## 更多资源

- [PostgreSQL 官方文档](https://www.postgresql.org/docs/)
- [SQLite 官方文档](https://www.sqlite.org/docs.html)
- [agix 配置参考](../config.md)
- [agix CLI 参考](../cli/)
