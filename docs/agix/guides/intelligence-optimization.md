# 智能优化

## 概述

agix 包含多个高级功能来优化 LLM 请求处理、降低成本、改善性能：

- **智能路由** — 自动为简单请求使用更廉价的模型
- **语义缓存** — 缓存相似提示的响应，避免冗余 LLM 调用
- **上下文压缩** — 对话过长时自动总结旧消息
- **A/B 测试** — 运行流量分割实验以比较模型性能

## 智能路由

智能路由自动将简单请求路由到更廉价的模型，同时将复杂请求保留在高端模型上。

### 工作原理

1. 请求从 Agent 到达
2. Proxy 分析请求：
   - 对话中的消息数
   - 输入 Token 总数
   - 消息复杂度启发式
3. 分类为"简单"或"复杂"
4. 根据分类路由到相应的模型

### 配置

启用并配置路由：

```yaml
routing:
  enabled: true

  tiers:
    simple:
      max_message_tokens: 500      # 消息低于 500 tokens
      max_messages: 3              # 对话 ≤3 条消息

  model_map:
    gpt-4o:
      simple: "gpt-4o-mini"        # 简单请求 → mini
      complex: "gpt-4o"            # 复杂请求 → 完整模型

    claude-opus-4-6:
      simple: "claude-haiku-4-5-20251001"
      complex: "claude-opus-4-6"
```

### 成本影响示例

场景：代码审查 Agent 每日处理 100 个请求

- **不使用路由**：100 × gpt-4o（每次请求 $0.003）= $0.30/天
- **使用路由**：
  - 60 个简单 → gpt-4o-mini（$0.00015 每次）= $0.009
  - 40 个复杂 → gpt-4o = $0.12
  - **总计：$0.129/天（节省 57%）**

### 监控路由决策

遗憾的是 agix 目前不在日志中暴露路由决策，但你可以推断：

1. 按模型过滤日志
2. 注意模式：简单请求 → mini，复杂 → 完整
3. 根据需要调整层级阈值

## 语义缓存

语义缓存使用 embeddings 查找相似的缓存响应，而不是重新查询 LLM。

### 工作原理

1. 带有提示的请求到达
2. Proxy 为提示生成 embedding
3. 搜索缓存中的相似 embeddings（余弦相似度 > 阈值）
4. 如果找到匹配：返回缓存响应（节省$$和延迟）
5. 如果没有匹配：转发给 LLM，缓存结果

### 配置

```yaml
cache:
  enabled: true
  similarity_threshold: 0.95       # 0-1，相似度（1=精确）
  ttl_minutes: 60                  # 缓存 60 分钟后过期
```

### 何时使用

**适合缓存的用例：**
- 常见问题或常见问题
- 文档查询 Agent
- 基于模板的响应
- 重复的总结任务

**不适合缓存的用例：**
- 高度动态的内容（新闻、实时信息）
- 个性化响应（不同用户）
- 创意/生成任务
- 实时分析

### 响应头

缓存命中在响应头中指示：

```
X-Cache: HIT        # 来自缓存的响应
X-Cache: MISS       # 全新 LLM 响应
```

### 缓存命中示例

```bash
# 第一次请求（缓存未命中）
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "X-Agent-Name: doc-agent" \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "Explain OAuth"}]}'
# 响应：X-Cache: MISS, X-Cost-USD: 0.15

# 第二次类似请求（缓存命中）
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "X-Agent-Name: doc-agent" \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "What is OAuth?"}]}'
# 响应：X-Cache: HIT, X-Cost-USD: 0.00
```

### 调整相似度阈值

- **0.99-1.0**：仅缓存精确匹配（低命中率，安全）
- **0.95-0.99**：需要高度相似（好的默认值，最小幻觉风险）
- **0.90-0.95**：更激进的缓存（监控错误答案）
- **<0.90**：不推荐（可能返回无关的缓存响应）

## 上下文压缩

对于长时间运行的对话，上下文窗口会填满。上下文压缩自动总结旧消息以释放空间。

### 工作原理

1. 请求到达，包含 N 条消息
2. Proxy 检查：total_tokens > compression_threshold？
3. 如果是：
   - 选择最旧的消息（保留最近的）
   - 使用轻量级模型进行总结
   - 用摘要替换原始消息
4. 转发压缩后的对话给 LLM

### 配置

```yaml
compression:
  enabled: true
  threshold_tokens: 50000          # 对话超过 50k tokens 时触发
  keep_recent: 10                  # 始终保留最近 N 条消息
  summary_model: "gpt-4o-mini"     # 用于总结的模型
```

### 触发时机

典型示例：

```
初始状态：
- 消息 1："告诉我 X"
- 消息 2："解释 Y"
- ...
- 消息 25："现在 Z 如何工作？"
- 总计：52,000 tokens

应用压缩后：
- 摘要："用户询问了 X、Y 和其他主题"
- 消息 24：（原样保留）
- 消息 25：（原样保留）
- 总计：35,000 tokens
```

### 成本权衡

压缩产生较小成本（总结 LLM 调用），但在主 LLM 上节省 Token：

```
不使用压缩：
- 请求 tokens：50,000
- 成本：50,000 × $0.005 = $0.25

使用压缩：
- 总结：5,000 → 500 tokens = $0.0025
- 请求 tokens：35,000（带摘要）
- 成本：35,000 × $0.005 + $0.0025 = $0.1775
- 节省：29%
```

## A/B 测试

运行流量分割实验以比较模型性能。

### 工作原理

1. 定义带有两个模型的实验
2. 指定流量分割（例如 20% 变体，80% 对照）
3. 根据 Agent ID 确定性地路由请求
4. 同一 Agent 始终获得同一变体（一致体验）
5. 不同 Agent 看到不同变体（流量分割）

### 配置

```yaml
experiments:
  - name: "test-gpt4o-vs-mini"
    enabled: true
    control_model: "gpt-4o"
    variant_model: "gpt-4o-mini"
    traffic_pct: 20                # 20% 到变体，80% 到对照

  - name: "claude-sonnet-vs-opus"
    enabled: true
    control_model: "claude-opus-4-6"
    variant_model: "claude-sonnet-4-5-20250929"
    traffic_pct: 50                # 50/50 分割
```

### 检查变体分配

```bash
# 检查 Agent 分配到哪个变体
agix experiment check code-reviewer gpt-4o
# 输出：code-reviewer → variant (gpt-4o-mini)

agix experiment check docs-writer gpt-4o
# 输出：docs-writer → control (gpt-4o)
```

### 确定性分配

分配基于 hash(agent_name + model)：

```
同一 Agent + 模型组合 = 始终相同的变体
不同 Agent = 可能不同的变体
```

### 分析模式

手动跟踪实验结果：

1. 运行实验 N 天
2. 导出成本和日志：
   ```bash
   agix logs --agent code-reviewer -n 1000 | grep "gpt-4o"
   agix logs --agent docs-writer -n 1000 | grep "gpt-4o"
   ```
3. 比较：
   - 成本差异
   - 质量（手动审查或质量门警告）
   - 速度（来自日志的 duration_ms）

### 真实示例

测试 gpt-4o vs gpt-4o-mini 用于代码审查：

```yaml
experiments:
  - name: "code-review-cost-test"
    enabled: true
    control_model: "gpt-4o"           # 基线（昂贵）
    variant_model: "gpt-4o-mini"      # 廉价选项
    traffic_pct: 50                   # 50% 的 Agent 尝试 mini

# 一周后：
# - Mini 变体 Agent：10,000 个审查，成本=$45，质量=98%
# - 完整变体 Agent：10,000 个审查，成本=$150，质量=99%
#
# 决定：1% 质量提升不值得 3 倍成本
# → 将所有代码审查器切换到 gpt-4o-mini
```

## 组合功能

### 示例：成本优化管道

```yaml
# 1. 智能路由实现廉价简单请求
routing:
  enabled: true
  tiers:
    simple:
      max_message_tokens: 500
      max_messages: 3
  model_map:
    gpt-4o:
      simple: "gpt-4o-mini"        # 廉价
      complex: "gpt-4o"            # 昂贵

# 2. 缓存常见响应
cache:
  enabled: true
  similarity_threshold: 0.95
  ttl_minutes: 120                 # 保留较长时间（6 小时）

# 3. 在长对话中压缩旧消息
compression:
  enabled: true
  threshold_tokens: 40000
  keep_recent: 8
  summary_model: "gpt-4o-mini"

# 4. 通过 A/B 测试新的廉价模型
experiments:
  - name: "all-mini-experiment"
    enabled: true
    control_model: "gpt-4o"
    variant_model: "gpt-4o-mini"
    traffic_pct: 30                # 30% 尝试廉价选项
```

**结果**：成本降低 40-60%，同时保持质量。

## 最佳实践

1. **从智能路由开始** — 最容易的胜利，风险最低
2. **监控缓存命中率** — 根据命中率优化 `similarity_threshold`
3. **测试压缩** — 在生产前验证摘要仍然有用
4. **A/B 测试模型** — 始终验证廉价模型适用于你的用例
5. **组合功能** — 路由 + 缓存 + 压缩实现最大节省
6. **跟踪质量** — 启用质量门以早期捕获问题
