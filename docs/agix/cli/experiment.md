# experiment

## `agix experiment`

管理 A/B 测试实验。agix 支持在不修改 Agent 代码的情况下，将一部分流量路由到实验模型，对比两个模型的用量和费用。

```bash
agix experiment list                         # 列出所有已配置的实验
agix experiment check <agent> <model>        # 查看某 Agent 请求会命中哪个变体
```

## A/B 测试工作流

### 第一步：在 `config.yaml` 中配置实验

```yaml
experiments:
  - name: sonnet-vs-haiku
    enabled: true
    control_model: claude-sonnet-4-6          # 对照组（原始模型）
    variant_model: claude-haiku-4-5-20251001  # 实验组
    traffic_pct: 30                           # 30% 流量路由到实验组
```

### 第二步：验证实验配置

```bash
agix experiment list
```

输出：

```
 NAME              ENABLED  CONTROL                VARIANT                            TRAFFIC %
 sonnet-vs-haiku   yes      claude-sonnet-4-6      claude-haiku-4-5-20251001          30%
```

### 第三步：确认 Agent 分配结果

```bash
agix experiment check my-agent claude-sonnet-4-6
```

命中实验组：

```
Experiment: sonnet-vs-haiku
Variant:    variant
Model:      claude-haiku-4-5-20251001
```

命中对照组：

```
Experiment: sonnet-vs-haiku
Variant:    control
Model:      claude-sonnet-4-6
```

### 第四步：对比结果

通过 `agix stats --by model` 对比两组的 token 用量和费用，评估实验效果。

## 命令参考

### `experiment list` 列说明

| 列 | 说明 |
|----|------|
| NAME | 实验名称（`config.yaml` 中定义） |
| ENABLED | 是否启用（`yes` / `no`） |
| CONTROL | 对照组模型（Agent 发送原始 model 时使用） |
| VARIANT | 实验组模型 |
| TRAFFIC % | 路由到实验组的流量比例 |

### `experiment check` 输出字段

| 字段 | 说明 |
|------|------|
| Experiment | 匹配的实验名称 |
| Variant | 分配结果：`control`（对照组）或 `variant`（实验组） |
| Model | 实际使用的模型名称 |

## 分配机制

分配基于 `(agent_name, model)` 的哈希值确定性计算。同一 Agent 请求同一模型时**始终命中同一变体**，保证实验过程中 Agent 行为的一致性，避免结果污染。
