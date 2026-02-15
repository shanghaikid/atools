# 安装与使用

## 安装 ainit

```bash
cd tools/ainit
make install    # 构建并安装到 /usr/local/bin
```

运行安装：

```bash
ainit
```

输出：

```
  ~/.claude/commands/ainit.md
  ~/.claude/ainit-templates/

Installed. Run /ainit in any project to set up multi-agent collaboration.
```

## 在项目中使用

安装完成后，在任意项目的 Claude Code 会话中运行：

```
/ainit
```

这会触发 `ainit-setup.sh` 脚本，在当前项目中设置：

1. **CLAUDE.md** —— 项目级 Claude Code 配置（包含 backlog 协议）
2. **backlog.json** —— 需求管理索引
3. **backlog/** —— Story 详情目录
4. **workflow.md** —— 工作流说明

## 工作流

初始化后的典型工作流：

1. **创建 Story** —— team-lead 在 backlog.json 中创建需求
2. **设计阶段** —— architect 编写技术方案
3. **实现阶段** —— coder 按照设计方案编码
4. **审查阶段** —— reviewer 审查代码
5. **测试阶段** —— tester 验证功能
6. **完成** —— 合并到主分支

每个阶段的状态变更都记录在 Story 的 `audit_log` 中，保证完整可追溯。

## Backlog 协议

ainit 使用双文件结构管理需求：

- **`backlog.json`** —— 轻量级索引，只存储 id/title/status/branch
- **`backlog/STORY-N.json`** —— 完整的 Story 详情

这种设计让每个 Agent 只需要读取一个文件就能获取完整上下文，避免了读取大量文件的开销。
