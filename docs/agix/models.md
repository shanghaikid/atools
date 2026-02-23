# 模型与定价

agix 内置了主流 LLM 提供商的完整定价表，自动追踪每次请求的 Token 消耗与费用。代理会对请求中的模型名称进行**最长前缀匹配**，因此即使请求使用的是带日期后缀的版本化模型名，也能正确计费。

## 内置模型定价表

所有价格均为 **USD / 1M tokens**，数据来自各官方定价页面。

### OpenAI — GPT-5 系列

| 模型 | 输入价格 | 输出价格 | 说明 |
|------|---------|---------|------|
| `gpt-5.2` | $1.75 | $14.00 | 旗舰版本 |
| `gpt-5.1` | $1.25 | $10.00 | 标准版本 |
| `gpt-5` | $1.25 | $10.00 | 同 gpt-5.1 |
| `gpt-5-mini` | $0.25 | $2.00 | 轻量版 |
| `gpt-5-nano` | $0.05 | $0.40 | 极速版 |

### OpenAI — GPT-4 系列

| 模型 | 输入价格 | 输出价格 | 说明 |
|------|---------|---------|------|
| `gpt-4.1` | $2.00 | $8.00 | 最新 GPT-4 |
| `gpt-4.1-mini` | $0.40 | $1.60 | 轻量版 |
| `gpt-4.1-nano` | $0.10 | $0.40 | 极速版 |
| `gpt-4o` | $2.50 | $10.00 | 多模态旗舰 |
| `gpt-4o-mini` | $0.15 | $0.60 | 性价比优选 |

### OpenAI — 推理模型（Reasoning）

| 模型 | 输入价格 | 输出价格 | 说明 |
|------|---------|---------|------|
| `o1` | $15.00 | $60.00 | 深度推理 |
| `o3` | $2.00 | $8.00 | 高性价比推理 |
| `o3-mini` | $1.10 | $4.40 | 轻量推理 |
| `o4-mini` | $1.10 | $4.40 | 最新轻量推理 |

### Anthropic — Claude 4.x 系列

| 模型 | 输入价格 | 输出价格 | 说明 |
|------|---------|---------|------|
| `claude-opus-4-6` | $5.00 | $25.00 | 最新旗舰 |
| `claude-opus-4-5-20251101` | $5.00 | $25.00 | 旗舰稳定版 |
| `claude-opus-4-1-20250805` | $15.00 | $75.00 | 上一代旗舰 |
| `claude-opus-4-20250514` | $15.00 | $75.00 | 旗舰初版 |
| `claude-sonnet-4-5-20250929` | $3.00 | $15.00 | 平衡之选 |
| `claude-sonnet-4-20250514` | $3.00 | $15.00 | Sonnet 初版 |
| `claude-haiku-4-5-20251001` | $1.00 | $5.00 | 高速低价 |

### Anthropic — 历史模型

| 模型 | 输入价格 | 输出价格 | 说明 |
|------|---------|---------|------|
| `claude-3-5-haiku-20241022` | $0.80 | $4.00 | Claude 3.5 Haiku |
| `claude-3-haiku-20240307` | $0.25 | $1.25 | Claude 3 Haiku |

### DeepSeek

| 模型 | 输入价格 | 输出价格 | 说明 |
|------|---------|---------|------|
| `deepseek-chat` | $0.27 | $1.10 | 通用对话 |
| `deepseek-reasoner` | $0.55 | $2.19 | 推理增强版 |

> DeepSeek 使用与 OpenAI 兼容的 API，只需在配置中添加 DeepSeek API Key 即可使用：
> ```yaml
> keys:
>   deepseek: "sk-..."
> ```

## 模型名称匹配规则

agix 支持**最长前缀匹配**（Longest Prefix Match），这意味着：

```
gpt-4o-2024-08-06  →  匹配 "gpt-4o"（而非 "gpt-4"）
claude-opus-4-6-20260101  →  匹配 "claude-opus-4-6"
deepseek-chat-v3  →  匹配 "deepseek-chat"
```

**匹配逻辑**（来自 `internal/pricing/models.go`）：

1. 先做**精确匹配**：如果请求的模型名与定价表中某个 key 完全一致，直接返回该定价
2. 若精确匹配失败，遍历所有已知模型名，找出**最长的**作为 `HasPrefix` 前缀的那个
3. 所有匹配不区分大小写（统一转换为小写）
4. 若无任何匹配，成本计为 `$0.00`（请求仍正常代理，但不计费）

**为什么选最长前缀？**

避免 `"gpt-4"` 匹配上 `"gpt-4o-..."` 这类情况：`gpt-4o` 比 `gpt-4` 长，因此优先命中 `gpt-4o` 的定价。

**提供商识别**（`ProviderForModel`）

模型名前缀同样用于判断请求应发往哪个上游：

| 前缀 | 提供商 |
|------|--------|
| `gpt-`, `o1`, `o3`, `o4` | OpenAI |
| `claude-` | Anthropic |
| `deepseek-` | DeepSeek |
| 其他 | 查定价表；未知则为 `unknown` |

## 未知模型处理

若请求的模型不在定价表中：

- 请求**正常转发**到上游（不会被拦截）
- Token 消耗正常记录
- 费用计为 `$0.00`
- `agix stats` 中会显示该模型名，成本列为空

这种"fail-open"设计确保新模型上线时不影响代理功能，只是暂时无法计费。

## 添加自定义模型定价

agix 的定价表内置于 `internal/pricing/models.go`。目前**不支持**通过配置文件添加自定义定价。

如需为私有模型或新发布的模型添加定价，需修改源码后重新编译：

**步骤：**

1. 打开 `agix/internal/pricing/models.go`
2. 在 `models` map 中添加新条目：

```go
// 示例：添加一个假想的 my-llm 模型
"my-llm-v1": {Provider: "my-provider", InputPer1M: 1.00, OutputPer1M: 4.00},
```

3. 如果是全新提供商，同时更新 `ProviderForModel` 函数：

```go
case strings.HasPrefix(model, "my-llm-"):
    return "my-provider"
```

4. 重新编译：

```bash
cd agix
make install
```

> 自定义提供商的 API Key 通过 `keys` 配置项注入，与内置提供商相同：
> ```yaml
> keys:
>   my-provider: "sk-my-api-key"
> ```

## 模型选型建议

### 按任务场景

| 场景 | 推荐模型 | 理由 |
|------|---------|------|
| 代码审查、复杂推理 | `claude-opus-4-6` | 逻辑严密，上下文理解强 |
| 日常对话、内容生成 | `claude-sonnet-4-5-20250929` | 质量与成本平衡最优 |
| 高频批处理、简单分类 | `claude-haiku-4-5-20251001` | 速度最快，成本最低 |
| 数学/逻辑推理题 | `o3` / `o4-mini` | 专为推理优化 |
| 多模态（图片理解）| `gpt-4o` | 原生多模态支持 |
| 超低成本大规模请求 | `deepseek-chat` | $0.27/$1.10，性价比极高 |

### 按成本区间

**经济型（输入 < $0.30/1M）**

- `gpt-5-nano` — $0.05 / $0.40
- `claude-3-haiku-20240307` — $0.25 / $1.25
- `deepseek-chat` — $0.27 / $1.10
- `gpt-5-mini` — $0.25 / $2.00

**标准型（输入 $0.30–$3.00/1M）**

- `gpt-4o-mini` — $0.15 / $0.60
- `claude-haiku-4-5-20251001` — $1.00 / $5.00
- `gpt-4.1-mini` — $0.40 / $1.60
- `claude-sonnet-4-5-20250929` — $3.00 / $15.00

**旗舰型（输入 > $3.00/1M）**

- `claude-opus-4-6` — $5.00 / $25.00
- `gpt-4o` — $2.50 / $10.00
- `o1` — $15.00 / $60.00（推理专用）

### 结合 agix 功能降低成本

1. **智能路由**：简单请求自动降级到低价模型

   ```yaml
   routing:
     enabled: true
     tiers:
       simple:
         max_message_tokens: 500
         max_messages: 3
     model_map:
       claude-opus-4-6: { simple: claude-haiku-4-5-20251001 }
   ```

2. **语义缓存**：相似请求直接返回缓存，零成本

   ```yaml
   cache:
     enabled: true
     similarity_threshold: 0.95
     ttl_minutes: 60
   ```

3. **A/B 测试**：用小流量验证低价模型效果后再全量切换

   ```yaml
   experiments:
     - name: haiku-experiment
       enabled: true
       control_model: claude-sonnet-4-5-20250929
       variant_model: claude-haiku-4-5-20251001
       traffic_pct: 20
   ```

4. **预算硬限制**：防止意外超支

   ```yaml
   budgets:
     my-agent:
       daily_limit_usd: 5.0
       monthly_limit_usd: 100.0
   ```

## 查看实时定价统计

```bash
# 按模型分组查看成本
agix stats --group-by model

# 查看今日消耗
agix stats

# 查看指定月份
agix stats --period 2026-02
```
