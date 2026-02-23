# MCP 工具深度文档

本指南深入讲解 Model Context Protocol（MCP）工具在 agix 中的实现原理，包括如何编写自定义 MCP Server、调试工具调用、处理错误等企业级主题。

## MCP 协议基础

agix 使用 **MCP 2024-11-05** 版本，基于 JSON-RPC 2.0 标准。

### 消息格式

所有通信都是基于行的 JSON，每行一条消息，以 `\n` 分隔：

```json
{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {...}}
{"jsonrpc": "2.0", "id": 1, "result": {...}}
```

### 核心数据结构

**Tool 定义**
```json
{
  "name": "read_file",
  "description": "读取文件内容",
  "inputSchema": {
    "type": "object",
    "properties": {
      "path": {
        "type": "string",
        "description": "文件路径"
      }
    },
    "required": ["path"]
  }
}
```

**ToolResult 结果**
```json
{
  "content": [
    {
      "type": "text",
      "text": "文件内容"
    }
  ],
  "isError": false
}
```

### 核心 RPC 方法

| 方法 | 参数 | 返回 | 说明 |
|------|------|------|------|
| `initialize` | `protocolVersion`, `capabilities`, `clientInfo` | `serverInfo`, `capabilities` | 握手协议，必需首先调用 |
| `notifications/initialized` | 无 | 无应答 | 初始化完成通知 |
| `tools/list` | 无 | `{ tools: [...] }` | 列举所有可用工具 |
| `tools/call` | `{ name, arguments }` | `{ content, isError }` | 执行工具 |

## 自定义 MCP Server 开发

### 设计原则

编写 MCP Server 时遵循以下原则：

1. **无状态工具**：每个工具调用应该是独立的，不依赖全局状态
2. **快速响应**：工具应该在合理时间内返回（建议 < 30s）
3. **明确错误**：返回有意义的错误消息，帮助 Agent 理解发生了什么
4. **安全第一**：验证输入，避免路径遍历、注入等安全问题

### Node.js 实现示例

使用官方 `@modelcontextprotocol/sdk` 库编写 MCP Server：

```javascript
#!/usr/bin/env node

import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";

const server = new Server({
  name: "example-server",
  version: "1.0.0",
});

// 定义工具
server.setRequestHandler(
  "tools/list",
  async () => {
    return {
      tools: [
        {
          name: "greet",
          description: "问候某人",
          inputSchema: {
            type: "object",
            properties: {
              name: {
                type: "string",
                description: "要问候的人名",
              },
            },
            required: ["name"],
          },
        },
      ],
    };
  }
);

// 执行工具
server.setRequestHandler(
  "tools/call",
  async (request) => {
    const { name, arguments: args } = request.params;

    if (name === "greet") {
      return {
        content: [
          {
            type: "text",
            text: `你好，${args.name}！`,
          },
        ],
      };
    }

    return {
      content: [
        {
          type: "text",
          text: `未知工具：${name}`,
        },
      ],
      isError: true,
    };
  }
);

// 启动服务器
const transport = new StdioServerTransport();
await server.connect(transport);
```

在 `config.yaml` 中配置：

```yaml
tools:
  servers:
    example:
      command: "node"
      args: ["/path/to/example-server.js"]
```

### Go 实现示例

如果用 Go 编写 MCP Server：

```go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		var req jsonRPCRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}

		var result interface{}

		switch req.Method {
		case "initialize":
			result = map[string]interface{}{
				"serverInfo": map[string]string{
					"name":    "example-go",
					"version": "1.0.0",
				},
				"capabilities": map[string]interface{}{},
			}

		case "tools/list":
			result = map[string]interface{}{
				"tools": []map[string]interface{}{
					{
						"name":        "greet",
						"description": "问候某人",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"name": map[string]string{
									"type":        "string",
									"description": "要问候的人名",
								},
							},
							"required": []string{"name"},
						},
					},
				},
			}

		case "tools/call":
			params := req.Params.(map[string]interface{})
			toolName := params["name"].(string)
			args := params["arguments"].(map[string]interface{})

			if toolName == "greet" {
				result = map[string]interface{}{
					"content": []map[string]string{
						{
							"type": "text",
							"text": fmt.Sprintf("你好，%s！", args["name"]),
						},
					},
				}
			}

		case "notifications/initialized":
			// 无需响应
			continue
		}

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}
		respBytes, _ := json.Marshal(resp)
		fmt.Printf("%s\n", respBytes)
	}
}
```

### 输入验证最佳实践

**始终验证工具输入**，防止安全问题：

```javascript
server.setRequestHandler("tools/call", async (request) => {
  const { name, arguments: args } = request.params;

  if (name === "read_file") {
    // ❌ 错误：直接使用路径，容易路径遍历
    // const content = fs.readFileSync(args.path);

    // ✅ 正确：规范化路径，确保在允许范围内
    const path = require("path");
    const allowedDir = "/home/app/documents";
    const fullPath = path.resolve(allowedDir, args.path);

    if (!fullPath.startsWith(allowedDir)) {
      return {
        content: [
          {
            type: "text",
            text: "错误：路径不在允许范围内",
          },
        ],
        isError: true,
      };
    }

    try {
      const content = fs.readFileSync(fullPath, "utf-8");
      return {
        content: [{ type: "text", text: content }],
      };
    } catch (err) {
      return {
        content: [{ type: "text", text: `读取失败：${err.message}` }],
        isError: true,
      };
    }
  }

  return {
    content: [{ type: "text", text: `未知工具：${name}` }],
    isError: true,
  };
});
```

## 工具调试与排查

### 1. 启用调试日志

在 `config.yaml` 中设置日志级别：

```yaml
log_level: debug  # 记录所有 MCP 消息
```

启动 agix：

```bash
agix start --port 8080
# stderr 会输出 MCP 通信日志
```

日志示例：

```
[DEBUG] MCP client init: tools/list returned 5 tools
[DEBUG] Tool call: name=read_file arguments={"path": "/tmp/test.txt"}
[DEBUG] Tool result (read_file): 42 bytes, isError=false
```

### 2. 使用 agix trace 查看完整请求链路

```bash
# 查看最近的请求和它们的 tool_call 详情
agix trace list

# 查看单个 trace 的详细信息
agix trace <trace-id>

# 输出示例
# Trace ID: trace-abc123
# Agent: code-reviewer
# Model: claude-opus-4-6
# Duration: 2500ms
#
# Tool calls (in order):
#   1. read_file(path="/src/main.go") → 124 bytes, 50ms
#   2. read_file(path="/src/types.go") → 89 bytes, 40ms
#   3. format_response(text="reviewed code...") → success, 30ms
```

### 3. 查看 MCP Server 的 stderr 输出

MCP Server 的 stderr 直接转发到 agix 的 stderr：

```bash
# 启动 agix 并将输出重定向到文件
agix start 2> /tmp/agix-debug.log

# 查看 Server 日志（比如 npm Server）
tail -f /tmp/agix-debug.log
```

### 4. 独立测试 MCP Server

不启动 agix，直接测试 MCP Server：

```bash
# 启动 Server 并与其交互
npx -y @modelcontextprotocol/server-filesystem /tmp &
SERVER_PID=$!

# 发送 initialize 请求
echo '{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "test", "version": "1.0"}}}' | nc localhost 3000

kill $SERVER_PID
```

或者使用提供的测试工具（如果有）：

```bash
# 使用 @modelcontextprotocol/server-testing 库
npm install @modelcontextprotocol/server-testing
```

### 5. 常见调试问题

| 问题 | 症状 | 排查步骤 |
|------|------|------|
| **Server 不启动** | 超时或 "process start failed" | 1. 直接运行命令确保可执行。2. 检查环境变量是否正确（如 `GITHUB_TOKEN`）。3. 查看 stderr 是否有错误。 |
| **工具列表为空** | `tools/list` 返回 0 个工具 | 1. 确认 `tools/list` 被正确调用。2. 检查 Server 的 `inputSchema` 是否有效 JSON。3. 手动测试 Server 的 `tools/list` 响应。 |
| **工具调用超时** | 请求挂起 30 秒 | 1. 工具执行太慢（优化算法或加缓存）。2. Server 进程卡死（检查 Server 日志）。3. IO 阻塞（如大文件读取）。 |
| **工具结果格式错误** | "parse tools/call response" 错误 | 1. 检查返回的 JSON 是否有 `content` 字段。2. 确认 `content` 是数组。3. 每个 content block 必须有 `type` 和 `text` 字段。 |

## 工具调用失败处理

### 1. 错误如何传播

工具调用失败时，agix 的处理流程：

```
MCP Server 返回 tool result
    ↓
result.isError == true?
    ├─ 是 → 将错误消息作为工具结果附加到对话
    │       ↓
    │       LLM 重新请求（最多 max_iterations 次）
    │       LLM 可以选择：
    │       ├─ 重试（调整参数）
    │       ├─ 使用备选方案
    │       └─ 向用户报告错误
    │
    └─ 否 → 附加成功结果 → 继续工具循环
```

### 2. 修复工具错误的最佳实践

**定义清晰的错误消息**：

```javascript
// ❌ 不好：模糊的错误
return {
  content: [{ type: "text", text: "Error" }],
  isError: true,
};

// ✅ 好：具体的错误消息
return {
  content: [
    {
      type: "text",
      text: "文件读取失败：权限被拒绝。请确保文件可读或指定不同的路径。",
    },
  ],
  isError: true,
};
```

**包含故障排查建议**：

```javascript
if (!fs.existsSync(filePath)) {
  return {
    content: [
      {
        type: "text",
        text: `文件不存在：${filePath}。请尝试使用 'list_directory' 工具找到正确的文件名。`,
      },
    ],
    isError: true,
  };
}
```

### 3. 重试策略

agix 的重试行为由 LLM 决定。当工具返回错误时，LLM 可以：

- **重试同一工具**（调整参数）
- **使用不同的工具**（备选方案）
- **放弃工具使用**（向用户报告）

配置 `max_iterations` 限制重试次数：

```yaml
tools:
  max_iterations: 10  # 防止无限循环
```

### 4. 超时处理

工具调用超时时（默认 30s），agix 返回超时错误：

```
tool error: timeout after 30s waiting for response
```

LLM 将其视为工具失败，可以选择重试或放弃。

优化慢工具的建议：

1. **缓存结果**：为频繁调用的操作添加缓存
2. **分解操作**：将大任务拆分为多个小工具
3. **异步处理**：对于长时间操作，返回"已排队"并稍后轮询

```javascript
// 例子：分解大文件读取
{
  name: "read_file_lines",
  description: "读取文件的指定行范围（快速）",
  inputSchema: {
    type: "object",
    properties: {
      path: { type: "string" },
      start: { type: "number", description: "起始行号" },
      end: { type: "number", description: "结束行号" },
    },
    required: ["path", "start", "end"],
  },
}
```

### 5. 错误恢复模式

**修复-重试模式**：当工具失败时，Agent 可以自动修复和重试：

```
Agent 发送请求
    ↓
LLM 决定使用工具 X
    ↓
工具 X 失败（例如：权限不足）
    ↓
LLM 看到错误消息："权限被拒绝"
    ↓
LLM 理解问题并尝试备选方案
    ↓
使用工具 Y（例如：请求提升权限或使用备选路径）
    ↓
成功或继续循环
```

**Circuit breaker 模式**：避免重复失败：

```javascript
// 在 Server 中实现
const failureCount = {};

function recordFailure(toolName) {
  failureCount[toolName] = (failureCount[toolName] || 0) + 1;
  if (failureCount[toolName] > 3) {
    return {
      content: [
        {
          type: "text",
          text: `工具 ${toolName} 多次失败，已禁用。请检查配置。`,
        },
      ],
      isError: true,
    };
  }
}
```

## 工具发现与权限控制

### 工具列表查询

查看所有可用工具：

```bash
agix tools list

# 输出示例
# MCP Server: filesystem
#   read_file         读取文件内容
#   write_file        写入文件
#   list_directory    列举目录内容
#
# MCP Server: github
#   search_repos      搜索仓库
#   get_file          获取文件
#   create_pull_request  创建 PR
```

### 按 Agent 查看可用工具

```bash
# 查看特定 Agent 可用的工具（基于 allow/deny 配置）
agix tools list --agent code-reviewer

# 输出只包括 code-reviewer 有权限的工具
```

### 工具权限配置

在 `config.yaml` 中使用 `allow` 或 `deny` 控制访问：

```yaml
tools:
  servers:
    filesystem:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]

    github:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-github"]
      env: ["GITHUB_TOKEN=ghp_xxx"]

  agents:
    # 白名单模式：只允许这些工具
    code-reviewer:
      allow:
        - read_file
        - list_directory
        - search_repos

    # 黑名单模式：禁止这些工具
    untrusted-agent:
      deny:
        - delete_file
        - write_file
        - modify_repository

    # 通配符：允许所有工具（可信 Agent）
    trusted-agent:
      allow: ["*"]
```

## MCP JSON-RPC 协议细节

### 协议版本

agix 支持 MCP **2024-11-05** 版本。初始化时确认版本号：

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": {
      "name": "agix",
      "version": "1.0.0"
    }
  }
}
```

### 生命周期

1. **建立连接**：agix 启动 MCP Server 进程（使用 stdio）
2. **初始化握手**
   - 发送 `initialize` 请求
   - 接收服务器 `serverInfo` 和 `capabilities`
3. **发送初始化通知**：`notifications/initialized`
4. **发现工具**：`tools/list`
5. **执行工具**：每个请求时调用 `tools/call`
6. **关闭连接**：agix 停止时关闭 stdin，Server 收到 EOF，并收到 SIGINT

### 并发性

- **同一 Server 的调用序列化**：通过 mutex 确保不会同时发送两个请求到同一 Server
- **不同 Server 的调用并发**：多个 Server 可以同时执行工具

```
Agent 请求：使用工具 A（filesystem）和工具 B（github）
    ↓
agix 并发执行：
    ├─ filesystem.CallTool("read_file", ...) [mutex 1]
    └─ github.CallTool("search_repos", ...) [mutex 2]
    ↓
两个结果同时返回给 LLM
```

## 性能与监控

### 监控工具调用

使用 audit 日志追踪工具调用：

```bash
# 查看所有工具调用事件
agix audit list --type tool_call

# 输出示例
# 2026-02-23 10:30:45.123  tool_call   agent=code-reviewer    tool=read_file    status=success    duration_ms=45
# 2026-02-23 10:30:46.456  tool_call   agent=code-reviewer    tool=format_code  status=error      duration_ms=120
```

### 工具性能指标

检查工具的平均响应时间：

```bash
# 导出数据用于分析
agix export --format json | jq '.tool_calls[] | {name: .tool, duration: .duration_ms}'
```

### 调优建议

| 指标 | 目标 | 优化方法 |
|------|------|------|
| **平均延迟** | < 1s | 缓存频繁操作结果，优化算法 |
| **超时率** | < 1% | 增加超时限制或简化工具逻辑 |
| **错误率** | < 5% | 改善输入验证，添加重试逻辑 |
| **并发度** | 单 Server 串行，多 Server 并发 | 无需调优（由设计决定） |

## 常见场景

### 场景 1：文件操作工具

```javascript
const server = new Server({ name: "file-ops", version: "1.0" });

server.setRequestHandler("tools/list", async () => ({
  tools: [
    {
      name: "safe_read",
      description: "安全读取文件（限制于 /tmp）",
      inputSchema: {
        type: "object",
        properties: {
          path: { type: "string" },
          encoding: { type: "string", enum: ["utf-8", "base64"] },
        },
        required: ["path"],
      },
    },
    {
      name: "find_files",
      description: "按模式查找文件",
      inputSchema: {
        type: "object",
        properties: {
          directory: { type: "string" },
          pattern: { type: "string", description: "glob 模式" },
        },
        required: ["directory", "pattern"],
      },
    },
  ],
}));

server.setRequestHandler("tools/call", async (request) => {
  const { name, arguments: args } = request.params;

  try {
    if (name === "safe_read") {
      const path = require("path").resolve("/tmp", args.path);
      if (!path.startsWith("/tmp")) throw new Error("路径超出范围");

      const fs = require("fs");
      const content = fs.readFileSync(path, args.encoding || "utf-8");
      return { content: [{ type: "text", text: content }] };
    }

    if (name === "find_files") {
      const glob = require("glob");
      const files = glob.sync(args.pattern, { cwd: args.directory });
      return { content: [{ type: "text", text: JSON.stringify(files) }] };
    }
  } catch (err) {
    return {
      content: [{ type: "text", text: err.message }],
      isError: true,
    };
  }
});
```

### 场景 2：API 调用工具

```javascript
server.setRequestHandler("tools/call", async (request) => {
  const { name, arguments: args } = request.params;

  if (name === "api_call") {
    try {
      const response = await fetch(args.url, {
        method: args.method || "GET",
        headers: args.headers || {},
        body: args.body ? JSON.stringify(args.body) : undefined,
      });

      if (!response.ok) {
        return {
          content: [
            {
              type: "text",
              text: `HTTP ${response.status}: ${response.statusText}`,
            },
          ],
          isError: true,
        };
      }

      const data = await response.json();
      return { content: [{ type: "text", text: JSON.stringify(data) }] };
    } catch (err) {
      return {
        content: [{ type: "text", text: `API 调用失败：${err.message}` }],
        isError: true,
      };
    }
  }
});
```

## 总结

- **MCP 是标准化的工具接口**：基于 JSON-RPC 2.0，允许任何语言编写 Server
- **设计简单工具**：每个工具做一件事，快速返回，明确错误
- **调试通过日志和 trace**：启用 debug 日志，使用 `agix trace` 查看详情
- **错误由 LLM 处理**：返回清晰的错误消息，让 LLM 决定如何恢复
- **权限控制保护安全**：使用 allow/deny 列表限制 Agent 访问
