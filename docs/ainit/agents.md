# Agent 模板

ainit 提供六种预定义的 Agent 角色，每种角色有明确的职责和读写权限。

## 角色详情

### team-lead

项目管理者，负责全局视角。

- **职责**：需求分析、任务拆分、进度管理、状态同步
- **读权限**：backlog.json + 所有 Story 文件
- **写权限**：backlog.json 索引、Story 顶级字段、tasks、audit_log

### architect

系统设计师。

- **职责**：技术方案设计、架构决策
- **读权限**：分配的 Story 文件
- **写权限**：story.design、story.tasks

### coder

编码实现者。

- **职责**：按照设计方案编写代码
- **读权限**：分配的 Story 文件
- **写权限**：story.implementation、story.tasks 状态

### tester

测试工程师。

- **职责**：编写和执行测试
- **读权限**：分配的 Story 文件
- **写权限**：story.testing

### reviewer

代码审查者。

- **职责**：审查代码质量和正确性
- **读权限**：分配的 Story 文件 + 分支 diff
- **写权限**：story.review

### docs-sync

文档同步者。

- **职责**：在 Story 完成后同步更新文档
- **读权限**：status=done 的 Story 文件
- **写权限**：不修改 backlog，只更新项目文档

## 权限矩阵

| Agent | backlog.json | story.design | story.implementation | story.testing | story.review | story.tasks |
|-------|:---:|:---:|:---:|:---:|:---:|:---:|
| team-lead | R/W | R | R | R | R | R/W |
| architect | R | R/W | - | - | - | R/W |
| coder | R | R | R/W | - | - | status |
| tester | R | R | R | R/W | - | - |
| reviewer | R | R | R | R | R/W | - |
| docs-sync | R | R | R | R | R | - |
