# 项目总览

**Agent Platform Tools** 是一个 Go 语言 monorepo，包含多个独立的 CLI 工具，为 AI Agent 平台提供基础设施支持。

## 项目结构

```
tools/
├── agix/        # LLM 反向代理（Token 追踪、预算、MCP 工具）
├── ainit/       # 多 Agent 协作项目初始化器
└── worldtime/   # 终端世界时钟
```

每个工具都是独立的，拥有自己的 `go.mod`、`Makefile`，可以单独构建和安装。

## 技术栈

| 工具 | 语言 | 主要依赖 | 说明 |
|------|------|----------|------|
| agix | Go 1.26 | Cobra, SQLite (pure Go) | 单二进制，零 CGO |
| ainit | Go 1.22 | embed | 无外部依赖 |
| worldtime | Go 1.23 | stdlib only | 无外部依赖 |

## 通用构建命令

所有工具使用统一的 Makefile 目标：

```bash
make build      # 构建二进制
make install    # 构建并安装到 /usr/local/bin
make test       # 运行测试
make vet        # 代码检查
make clean      # 清理构建产物
```

## 设计原则

- **单二进制部署** —— 每个工具编译为一个独立可执行文件，无运行时依赖
- **零 CGO** —— 纯 Go 实现，支持交叉编译
- **Go 标准布局** —— `cmd/` 存放 CLI 入口，`internal/` 存放内部实现
- **表驱动测试** —— 统一的测试风格
