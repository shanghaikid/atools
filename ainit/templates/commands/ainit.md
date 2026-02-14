Initialize this project for multi-agent collaboration. Follow these steps exactly:

## Step 1: Create agent files

Read each file from `~/.claude/ainit-templates/agents/` and copy them to `.claude/agents/` in the current project directory. Create the `.claude/agents/` directory if it doesn't exist. The agent files are: `team-lead.md`, `architect.md`, `coder.md`, `tester.md`, `reviewer.md`, `docs-sync.md`.

## Step 2: Create backlog infrastructure

1. Create `backlog.json` in the project root:
```json
{"project": "<detected-project-name>", "current_sprint": 1, "last_story_id": 0, "stories": []}
```
Replace `<detected-project-name>` with the actual project name (from package.json, go.mod, pyproject.toml, or directory name).

2. Create the `backlog/` directory if it doesn't exist.

## Step 3: Copy workflow.md

Read `~/.claude/ainit-templates/workflow.md` and write it to `workflow.md` in the project root.

## Step 4: Update CLAUDE.md

Read `~/.claude/ainit-templates/backlog-protocol.md`. Then:

- If `CLAUDE.md` already exists: append the backlog protocol content to the end of the file (add two blank lines before appending)
- If `CLAUDE.md` does not exist: run `/init` first to generate it, then append the backlog protocol content

## Step 5: Confirm

Print a summary of all created/modified files. The project is now ready for multi-agent collaboration. Users can start by saying: "Use the team-lead agent to implement XXX feature".
