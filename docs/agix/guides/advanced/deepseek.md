# DeepSeek 提供商

agix 原生支持 DeepSeek，与 OpenAI、Anthropic 并列，无需额外插件。

## 配置

在 `config.yaml` 中添加 DeepSeek API Key：

```yaml
keys:
  openai: "sk-..."
  anthropic: "sk-ant-..."
  deepseek: "sk-..."       # 添加这一行
```

即可开始使用 DeepSeek 模型，无需其他改动。

## 支持的模型

| 模型 | 特点 | 适用场景 |
|---|---|---|
| `deepseek-chat` | 通用对话，价格低廉 | 日常任务、高频调用 |
| `deepseek-reasoner` | 强推理能力 | 复杂分析、数学推导 |

## 自动路由规则

agix 根据模型名称前缀自动路由到对应服务商：

```
gpt-*       → OpenAI      (https://api.openai.com)
claude-*    → Anthropic   (https://api.anthropic.com)
deepseek-*  → DeepSeek    (https://api.deepseek.com)
```

只需在请求中指定模型名：

```python
client = OpenAI(base_url="http://localhost:8080/v1", api_key="unused")

# 自动路由到 DeepSeek
response = client.chat.completions.create(
    model="deepseek-chat",
    messages=[{"role": "user", "content": "解释量子纠缠"}]
)
```

## 成本对比

以处理 2000 字文档为例（约 500 input tokens + 200 output tokens）：

| 提供商 | 模型 | 输入成本 | 输出成本 | 合计 |
|---|---|---|---|---|
| OpenAI | gpt-4o-mini | $0.075‰ | $0.120‰ | **$0.195‰** |
| Anthropic | claude-haiku-4-5 | $0.040‰ | $0.048‰ | **$0.088‰** |
| DeepSeek | deepseek-chat | $0.070‰ | $0.056‰ | **$0.126‰** |

> DeepSeek 相比 GPT-4o-mini 约便宜 35%，适合大批量、对质量要求适中的任务。

## 作为故障转移目标

DeepSeek 是优质的跨提供商故障转移选项：

```yaml
failover:
  chains:
    # OpenAI 主力 → 更便宜的 OpenAI → DeepSeek → Anthropic 兜底
    gpt-4o:
      - "gpt-4o-mini"
      - "deepseek-chat"
      - "claude-opus-4-6"

    # Anthropic 主力 → 更便宜的 Anthropic → DeepSeek 兜底
    claude-opus-4-6:
      - "claude-sonnet-4-5-20250929"
      - "deepseek-chat"
```

## 智能路由配合使用

将 DeepSeek 配置为简单任务的低成本路由目标：

```yaml
routing:
  enabled: true
  rules:
    - if: complexity == "low"
      use: "deepseek-chat"
    - if: complexity == "medium"
      use: "gpt-4o-mini"
    - if: complexity == "high"
      use: "gpt-4o"
```

## 注意事项

- DeepSeek 使用 OpenAI 兼容 API，agix 无需格式转换，直接透传请求
- DeepSeek 的 `deepseek-reasoner` 模型响应较慢，不适合低延迟场景
- 流式输出（SSE）完全支持
- Tool Call（MCP 工具循环）完全支持
