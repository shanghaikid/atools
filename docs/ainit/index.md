# ainit

多 Agent 协作项目初始化器 —— 一键安装 `/ainit` 命令和 Agent 模板，在任意 Claude Code 项目中启用团队协作工作流。

## 它做什么？

ainit 是一个轻量级安装器，只需运行一次。它会将以下内容安装到 `~/.claude/`：

- **`/ainit` 命令** —— 一个 Claude Code 自定义斜杠命令
- **Agent 模板** —— 预定义的多角色 Agent 模板

安装完成后，在任意项目中运行 `/ainit` 即可初始化多 Agent 协作环境。

## 预定义角色

| 角色 | 职责 |
|------|------|
| **team-lead** | 需求分析、任务拆分、进度管理 |
| **architect** | 系统设计、技术方案 |
| **coder** | 编码实现 |
| **tester** | 测试验证 |
| **reviewer** | 代码审查 |
| **docs-sync** | 文档同步 |

## 技术栈

| 组件 | 选择 |
|------|------|
| 语言 | Go 1.22 |
| 模板 | Go embed |
| 依赖 | 无外部依赖 |

## 安装结构

```
~/.claude/
├── commands/
│   └── ainit.md              # /ainit 命令定义
└── ainit-templates/
    ├── agents/               # Agent 模板文件
    │   ├── team-lead.md
    │   ├── architect.md
    │   ├── coder.md
    │   ├── tester.md
    │   ├── reviewer.md
    │   └── docs-sync.md
    ├── workflow.md           # 工作流说明
    ├── backlog-protocol.md   # Backlog 协议
    ├── backlog.mjs           # Backlog 管理脚本
    └── ainit-setup.sh        # 项目初始化脚本
```
