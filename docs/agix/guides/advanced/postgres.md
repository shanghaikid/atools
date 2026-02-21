# PostgreSQL 后端

agix 默认使用 SQLite，适合单机部署。当请求量增大或需要多实例部署时，可切换到 PostgreSQL。

## SQLite vs PostgreSQL 对比

| 功能 | SQLite | PostgreSQL |
|---|---|---|
| 配置复杂度 | 零配置（自动创建文件） | 需要数据库服务 |
| 并发写入 | 良好（WAL 模式） | 优秀 |
| 多实例共享 | ❌ | ✅ |
| 存储上限 | GB 级 | 无限制 |
| 备份方式 | 文件复制 | `pg_dump` |
| 流式复制 | ❌ | ✅ |
| 适用场景 | 单机 / < 100 万请求/天 | 多机 / 高并发 / 高可用 |

## 快速配置

### 1. 创建数据库

```bash
# 创建数据库和用户
createdb agix
psql postgres -c "CREATE USER agix WITH PASSWORD 'your-password';"
psql postgres -c "GRANT ALL PRIVILEGES ON DATABASE agix TO agix;"
```

### 2. 更新 config.yaml

```yaml
# 替换默认的 SQLite 路径
database: "postgres://agix:your-password@localhost:5432/agix?sslmode=disable"
```

### 3. 启动 agix

```bash
agix start
# agix 自动创建所有表和索引，无需手动执行 DDL
```

## 连接字符串格式

```
postgres://用户名:密码@主机:端口/数据库名?参数
```

**常用配置示例**：

```yaml
# 开发环境（无 SSL）
database: "postgres://agix:password@localhost:5432/agix?sslmode=disable"

# 生产环境（要求 SSL）
database: "postgres://agix:password@prod-db.example.com:5432/agix?sslmode=require"

# 生产环境（验证 SSL 证书）
database: "postgres://agix:password@prod-db.example.com:5432/agix?sslmode=verify-full&sslrootcert=/path/to/ca.pem"

# 通过 pgBouncer 连接池
database: "postgres://agix:password@pgbouncer.example.com:6432/agix?sslmode=disable"
```

## 多实例部署

多个 agix 实例共享同一 PostgreSQL 数据库，实现负载均衡：

```
[agix 实例 1] ─┐
[agix 实例 2] ─┼─→ PostgreSQL ← 统一数据存储
[agix 实例 3] ─┘
```

各实例配置相同的 `database` 连接字符串即可，agix 的写操作均为异步插入，并发安全。

## SQLite → PostgreSQL 迁移

```bash
# 1. 导出现有 SQLite 数据
agix export --format json > backup.json

# 2. 编辑配置文件，切换数据库
vim ~/.agix/config.yaml
# 将 database 字段改为 postgres:// 连接字符串

# 3. 重启 agix（自动建表）
agix start

# 4. 根据需要导入历史数据
# backup.json 中包含所有历史请求记录，可用自定义脚本写入
```

> **注意**：`agix export` 仅导出 `requests` 表数据。Session overrides、审计日志、Trace 等数据不在导出范围内。

## 性能调优

对于高请求量场景，可添加额外索引：

```sql
-- 按 Agent + 时间查询（stats 命令常用）
CREATE INDEX idx_requests_agent_timestamp
  ON requests (agent_name, timestamp DESC);

-- 按模型查询
CREATE INDEX idx_requests_model
  ON requests (model, timestamp DESC);

-- 更新统计信息（定期执行）
ANALYZE requests;
```

## 备份与恢复

```bash
# 全量备份
pg_dump -U agix -d agix -f backup.sql

# 压缩备份
pg_dump -U agix -d agix | gzip > backup_$(date +%Y%m%d).sql.gz

# 恢复
psql -U agix -d agix < backup.sql

# 使用 agix 内置导出（只含 requests 表，格式更友好）
agix export --format json --period 2026-02 > requests_202602.json
```

## 自动检测机制

agix 根据 `database` 字段的值自动判断使用哪个驱动：

| 前缀 | 驱动 |
|---|---|
| `postgres://` 或 `postgresql://` | PostgreSQL（`lib/pq`） |
| 其他（文件路径） | SQLite（`modernc.org/sqlite`） |

无需任何额外配置，切换数据库只需修改连接字符串。
