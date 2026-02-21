# budget

## `agix budget`

管理每个 Agent 的每日 / 每月费用上限。超出预算时代理返回 `429 Too Many Requests`。

```bash
agix budget list                                       # 查看所有已配置的预算
agix budget set <agent> --daily 10.0 --monthly 200.0  # 设置预算
agix budget remove <agent>                             # 移除预算
```

### 子命令

#### `budget list`

列出所有 Agent 的预算设置及当前用量：

```
 AGENT           DAILY LIMIT  DAILY USED  MONTHLY LIMIT  MONTHLY USED
 code-reviewer   $10.00       $3.42       $200.00        $87.60
 docs-writer     $5.00        $1.10       $100.00        $23.40
```

#### `budget set <agent>`

为指定 Agent 设置或更新预算：

```bash
agix budget set code-reviewer \
  --daily 10.0 \
  --monthly 200.0 \
  --alert 80
```

| 选项 | 说明 |
|------|------|
| `--daily <USD>` | 每日费用上限（USD） |
| `--monthly <USD>` | 每月费用上限（USD） |
| `--alert <percent>` | 用量达到百分之几时触发预警（1-100，默认 80） |

#### `budget remove <agent>`

移除指定 Agent 的预算限制，之后该 Agent 不再受费用管控。

### 预算执行机制

- 预算检查在请求转发前执行，超额立即拒绝（fail-fast）
- 数据库查询失败时，代理**放行**请求（fail-open，不影响正常使用）
- 预算配置也可直接写在 `config.yaml` 的 `budgets:` 字段，CLI 命令与配置文件等效

详见[配置文件参考](/agix/config)中的 `budgets` 部分。
