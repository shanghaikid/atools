# 多语言集成示例

agix 完全兼容 OpenAI API 格式，因此所有支持 OpenAI 的 SDK 和框架均可零改动（或改动极小）地接入。只需将请求指向 `http://localhost:8080/v1` 即可。

## 环境变量配置（推荐方式）

多数 SDK 会自动读取环境变量，这是**零代码修改**接入 agix 的最简方式：

```bash
# 将所有 OpenAI 请求路由到 agix
export OPENAI_BASE_URL=http://localhost:8080/v1
export OPENAI_API_KEY=unused   # agix 会注入真实 key，此处随意填

# 可选：标识 Agent 身份（启用 per-agent 统计和预算）
export AGIX_AGENT_NAME=my-agent
```

> **说明**：`OPENAI_API_KEY` 必须设置（否则 SDK 会报错），但值无关紧要——agix 会用配置文件中的真实 key 替换它。

---

## Python（OpenAI SDK）

```bash
pip install openai
```

### 基础用法

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="unused",
    default_headers={"X-Agent-Name": "my-agent"},
)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "解释一下量子纠缠"}],
)
print(response.choices[0].message.content)
```

### 流式输出

```python
stream = client.chat.completions.create(
    model="claude-sonnet-4-5-20250929",
    messages=[{"role": "user", "content": "写一首关于夏天的诗"}],
    stream=True,
)
for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="", flush=True)
```

### 读取成本 Header

```python
import httpx
from openai import OpenAI

# 使用自定义 httpx client 以访问原始响应 header
http_client = httpx.Client()
client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="unused",
    http_client=http_client,
)

with client.chat.completions.with_raw_response.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Hello"}],
) as response:
    cost = response.headers.get("X-Cost-USD")
    tokens_in = response.headers.get("X-Input-Tokens")
    tokens_out = response.headers.get("X-Output-Tokens")
    print(f"Cost: ${cost}, In: {tokens_in}, Out: {tokens_out}")
    completion = response.parse()
    print(completion.choices[0].message.content)
```

### Anthropic 模型（通过 OpenAI 格式）

```python
# agix 自动将 OpenAI 格式转换为 Anthropic Messages API
response = client.chat.completions.create(
    model="claude-opus-4-6",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "分析这段代码的时间复杂度..."},
    ],
)
```

---

## Node.js / TypeScript

```bash
npm install openai
```

### 基础用法（TypeScript）

```typescript
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "http://localhost:8080/v1",
  apiKey: "unused",
  defaultHeaders: { "X-Agent-Name": "my-ts-agent" },
});

async function main() {
  const response = await client.chat.completions.create({
    model: "gpt-4o",
    messages: [{ role: "user", content: "你好！" }],
  });
  console.log(response.choices[0].message.content);
}

main();
```

### 流式输出

```typescript
const stream = await client.chat.completions.create({
  model: "claude-sonnet-4-5-20250929",
  messages: [{ role: "user", content: "请逐步解释 TCP 三次握手" }],
  stream: true,
});

for await (const chunk of stream) {
  const delta = chunk.choices[0]?.delta?.content ?? "";
  process.stdout.write(delta);
}
```

### 读取成本 Header（Node.js）

```typescript
const response = await client.chat.completions
  .create({
    model: "gpt-4o",
    messages: [{ role: "user", content: "Hello" }],
  })
  .withResponse();

const cost = response.response.headers.get("x-cost-usd");
const tokensIn = response.response.headers.get("x-input-tokens");
console.log(`Cost: $${cost}, Input tokens: ${tokensIn}`);
console.log(response.data.choices[0].message.content);
```

### CommonJS（.js / require）

```javascript
const OpenAI = require("openai");

const client = new OpenAI({
  baseURL: "http://localhost:8080/v1",
  apiKey: "unused",
});

client.chat.completions
  .create({
    model: "gpt-4o",
    messages: [{ role: "user", content: "Hello!" }],
  })
  .then((r) => console.log(r.choices[0].message.content));
```

---

## Go

agix 暴露标准的 OpenAI REST API，因此可以使用 [sashabaranov/go-openai](https://github.com/sashabaranov/go-openai) 或直接用 `net/http`。

### 使用 go-openai 库

```bash
go get github.com/sashabaranov/go-openai
```

```go
package main

import (
    "context"
    "fmt"
    "log"

    openai "github.com/sashabaranov/go-openai"
)

func main() {
    cfg := openai.DefaultConfig("unused") // agix 注入真实 key
    cfg.BaseURL = "http://localhost:8080/v1"

    client := openai.NewClientWithConfig(cfg)

    resp, err := client.CreateChatCompletion(
        context.Background(),
        openai.ChatCompletionRequest{
            Model: "gpt-4o",
            Messages: []openai.ChatCompletionMessage{
                {
                    Role:    openai.ChatMessageRoleUser,
                    Content: "用 Go 写一个并发安全的计数器",
                },
            },
        },
    )
    if err != nil {
        log.Fatalf("chat completion error: %v", err)
    }

    fmt.Println(resp.Choices[0].Message.Content)
}
```

### 传递 Agent 名称

```go
import "net/http"

// 使用自定义 Transport 注入 X-Agent-Name header
type agentTransport struct {
    base      http.RoundTripper
    agentName string
}

func (t *agentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    req = req.Clone(req.Context())
    req.Header.Set("X-Agent-Name", t.agentName)
    return t.base.RoundTrip(req)
}

// 构建带 header 的 HTTP client
httpClient := &http.Client{
    Transport: &agentTransport{
        base:      http.DefaultTransport,
        agentName: "my-go-agent",
    },
}

cfg := openai.DefaultConfig("unused")
cfg.BaseURL = "http://localhost:8080/v1"
cfg.HTTPClient = httpClient

client := openai.NewClientWithConfig(cfg)
```

### 读取成本 Header（原生 net/http）

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
)

type ChatRequest struct {
    Model    string    `json:"model"`
    Messages []Message `json:"messages"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

func main() {
    body, _ := json.Marshal(ChatRequest{
        Model:    "gpt-4o",
        Messages: []Message{{Role: "user", Content: "Hello!"}},
    })

    req, _ := http.NewRequest("POST", "http://localhost:8080/v1/chat/completions",
        bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer unused")
    req.Header.Set("X-Agent-Name", "my-go-agent")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    fmt.Printf("Cost: $%s\n", resp.Header.Get("X-Cost-USD"))
    fmt.Printf("Input tokens: %s\n", resp.Header.Get("X-Input-Tokens"))

    respBody, _ := io.ReadAll(resp.Body)
    fmt.Println(string(respBody))
}
```

---

## LangChain（Python）

```bash
pip install langchain langchain-openai
```

### 基础用法

```python
from langchain_openai import ChatOpenAI

llm = ChatOpenAI(
    model="gpt-4o",
    openai_api_base="http://localhost:8080/v1",
    openai_api_key="unused",
    default_headers={"X-Agent-Name": "langchain-agent"},
)

response = llm.invoke("解释一下 RAG（检索增强生成）的工作原理")
print(response.content)
```

### 使用 Anthropic 模型

```python
# LangChain 的 ChatOpenAI 也可以直接用 Anthropic 模型名
# agix 会自动路由到 Anthropic API
llm = ChatOpenAI(
    model="claude-sonnet-4-5-20250929",
    openai_api_base="http://localhost:8080/v1",
    openai_api_key="unused",
)
```

### 构建 Chain

```python
from langchain_core.prompts import ChatPromptTemplate
from langchain_openai import ChatOpenAI

llm = ChatOpenAI(
    model="gpt-4o",
    openai_api_base="http://localhost:8080/v1",
    openai_api_key="unused",
)

prompt = ChatPromptTemplate.from_messages([
    ("system", "你是一位代码审查专家，请简洁地指出代码问题。"),
    ("user", "{code}"),
])

chain = prompt | llm

result = chain.invoke({"code": "def add(a, b): return a + b + 1"})
print(result.content)
```

### 流式输出

```python
for chunk in llm.stream("逐步推导贝叶斯定理"):
    print(chunk.content, end="", flush=True)
```

### 带 ConversationBufferMemory 的对话

```python
from langchain.memory import ConversationBufferMemory
from langchain.chains import ConversationChain

memory = ConversationBufferMemory()
conversation = ConversationChain(llm=llm, memory=memory)

print(conversation.predict(input="我叫小明"))
print(conversation.predict(input="我叫什么名字？"))  # 应回答"小明"
```

---

## LlamaIndex（Python）

```bash
pip install llama-index llama-index-llms-openai
```

### 基础用法

```python
from llama_index.llms.openai import OpenAI
from llama_index.core import Settings

# 全局配置，影响所有 LlamaIndex 操作
Settings.llm = OpenAI(
    model="gpt-4o",
    api_base="http://localhost:8080/v1",
    api_key="unused",
    additional_kwargs={"headers": {"X-Agent-Name": "llamaindex-agent"}},
)

# 直接调用
from llama_index.core.llms import ChatMessage

response = Settings.llm.chat([
    ChatMessage(role="user", content="什么是向量数据库？"),
])
print(response.message.content)
```

### RAG（检索增强生成）Pipeline

```python
from llama_index.core import VectorStoreIndex, SimpleDirectoryReader
from llama_index.llms.openai import OpenAI
from llama_index.embeddings.openai import OpenAIEmbedding
from llama_index.core import Settings

# LLM 指向 agix
Settings.llm = OpenAI(
    model="gpt-4o",
    api_base="http://localhost:8080/v1",
    api_key="unused",
)

# Embedding 也可以指向 agix（如果 agix 代理了 embedding 端点）
# 或直接用 OpenAI embedding endpoint
Settings.embed_model = OpenAIEmbedding(
    api_base="http://localhost:8080/v1",
    api_key="unused",
)

# 构建索引
documents = SimpleDirectoryReader("./docs").load_data()
index = VectorStoreIndex.from_documents(documents)

# 查询
query_engine = index.as_query_engine()
response = query_engine.query("如何配置预算限制？")
print(response)
```

### 流式响应

```python
from llama_index.core.llms import ChatMessage

response_gen = Settings.llm.stream_chat([
    ChatMessage(role="user", content="解释 Transformer 架构"),
])
for token in response_gen:
    print(token.delta, end="", flush=True)
```

---

## curl / REST API 直接调用

无需任何 SDK，直接 HTTP 调用：

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer unused" \
  -H "X-Agent-Name: curl-test" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

查看响应 header（含成本）：

```bash
curl -si http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer unused" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"Hi"}]}' \
  | grep -E "^(X-Cost|X-Input|X-Output|HTTP)"
```

---

## 各语言环境变量对照表

| 语言/框架 | BASE_URL 变量 | API_KEY 变量 | Agent 名称 |
|-----------|-------------|------------|-----------|
| Python OpenAI SDK | `OPENAI_BASE_URL` | `OPENAI_API_KEY` | `default_headers` |
| Node.js OpenAI SDK | `OPENAI_BASE_URL` | `OPENAI_API_KEY` | `defaultHeaders` |
| Go go-openai | `cfg.BaseURL`（代码设置） | `cfg.APIKey`（代码设置） | 自定义 Transport |
| LangChain | `OPENAI_API_BASE` | `OPENAI_API_KEY` | `default_headers` |
| LlamaIndex | `api_base`（代码设置） | `OPENAI_API_KEY` | `additional_kwargs` |
| curl | `-H "..."` | `Authorization` header | `X-Agent-Name` header |

## 常见问题

**Q: SDK 报 "Invalid API Key" 怎么办？**

agix 不验证 API Key 格式，但有些 SDK 会在本地校验 key 格式（如必须以 `sk-` 开头）。如遇此问题，将 key 设为 `sk-unused` 即可。

**Q: 如何同时接入多个 Agent？**

每个 Agent 进程设置自己的 `X-Agent-Name` header，agix 会分别追踪它们的用量和预算。无需做任何其他配置。

**Q: 流式请求能追踪成本吗？**

可以。agix 解析 SSE 流中的 usage 数据块（OpenAI 在最后一个 chunk 发送，Anthropic 在 `message_stop` 事件中发送），并在流结束后记录完整成本。

**Q: 能不能代理 Embedding 请求？**

目前 agix 只代理 `/v1/chat/completions` 端点，不支持 `/v1/embeddings`。Embedding 请求需直接发往上游提供商。
