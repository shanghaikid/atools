# 系统提示词注入

系统提示词注入允许在代理层统一注入全局或按 Agent 的系统提示词，无需修改任何 Agent 代码。

## 工作原理

```
请求到达 agix
    ↓
选择提示词（全局 / 按 Agent）
    ↓
前置或后置到现有系统消息
    ↓
转发修改后的请求到 LLM
```

注入在防火墙扫描之后、语义缓存查找之前执行，因此注入的提示词会参与缓存 key 计算。

## 基础配置

```yaml
prompt_templates:
  enabled: true

  # 适用于所有 Agent 的全局模板
  global: "你是一个有帮助的助手。遵循公司政策。"

  # 按 Agent 的覆盖（优先级高于 global）
  agents:
    code-reviewer: "你是专家代码审查官。关注安全性和性能。"
    docs-writer: "你是技术文档编辑。使用清晰、简洁的语言。"
    compliance-checker: "你是合规官。根据政策检查所有响应。"

  # 注入位置：prepend（系统消息前）或 append（系统消息后）
  position: "prepend"
```

## 注入位置：Prepend vs Append

| 位置 | 说明 | 适用场景 |
|---|---|---|
| `prepend`（默认）| 注入内容在 LLM 推理中有更高优先级 | 强制性策略、合规要求 |
| `append` | 用户原有系统消息有更高优先级 | 软性指南、补充说明 |

```yaml
agents:
  strict-agent:
    template: "严格遵守以下规则：..."
    position: "prepend"    # 强制执行

  flexible-agent:
    template: "尽量帮助，不要太无聊。"
    position: "append"     # 软性建议
```

## 真实示例：企业合规策略

```yaml
prompt_templates:
  enabled: true

  global: |
    你是公司 AI 助手。

    重要规则：
    - 不推荐竞争对手产品
    - 遵守数据隐私法规（GDPR、CCPA）
    - 不参与政治讨论
    - 输出内容需符合品牌语气规范

  agents:
    customer-support:
      template: |
        你是友好的客户支持 Agent。
        优先考虑客户满意度。
        提供最佳方案，不追求最高利润。
      position: "prepend"

    sales-agent:
      template: |
        你是销售助手。
        推荐适合客户需求的公司产品。
        突显竞争优势，但不贬低竞争对手。
```

## 对成本的影响

注入的提示词增加每次请求的 Token 用量：

```
未注入：
  用户消息：100 tokens
  LLM 处理：100 tokens

注入后（global = 50 tokens）：
  系统提示：50 tokens
  用户消息：100 tokens
  LLM 处理：150 tokens → 成本增加 50%
```

**建议**：
- 保持全局提示词简洁（< 100 tokens）
- 使用 `agix stats` 监控 Token 用量变化
- 对高频 Agent 单独评估注入的成本影响

## 注意事项

- 注入仅对 OpenAI 兼容格式生效（在 Anthropic 格式转换前执行）
- Agent 完全无感知——注入操作对 Agent 透明
- `agents` 中的配置优先级高于 `global`，两者可叠加使用
