# 快速开始

## 安装

```bash
cd tools/agix
make install    # 构建并安装到 /usr/local/bin
```

或手动构建：

```bash
go build -o agix .
```

交叉编译：

```bash
GOOS=linux GOARCH=amd64 go build -o agix-linux .
GOOS=darwin GOARCH=arm64 go build -o agix-darwin .
```

## 初始化

```bash
agix init
```

这会在 `~/.agix/` 下创建配置文件 `config.yaml`。编辑它填入你的 API Key：

```yaml
port: 8080
keys:
  openai: "sk-..."
  anthropic: "sk-ant-..."
database: "/Users/you/.agix/agix.db"
```

## 启动代理

```bash
agix start
# 或指定端口
agix start --port 9090
```

## 接入 Agent

只需修改一行代码，将 Agent 的请求路由到 agix：

### Python (OpenAI SDK)

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="unused",  # agix 会注入真实的 key
    default_headers={"X-Agent-Name": "my-agent"},
)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Hello!"}],
)
```

### 环境变量（零代码修改）

```bash
export OPENAI_BASE_URL=http://localhost:8080/v1
```

## 查看统计

```bash
agix stats              # 今日总览
agix stats --by agent   # 按 Agent 分组
agix stats --by model   # 按模型分组
```
