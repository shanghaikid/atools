# ainit

> Multi-Agent Collaboration Project Initializer

## Tech Stack

- Language: Go 1.22
- Framework: None
- Database: None

## Directory Structure

```
├── main.go         # entrypoint (cobra CLI)
├── templates/      # embedded agent/skill/workflow templates
│   ├── agents/     # agent .md files
│   ├── commands/   # slash command templates
│   ├── skills/     # skill templates
│   ├── ainit-setup.sh
│   ├── backlog.mjs
│   ├── backlog-protocol.md
│   ├── backlog-schema.md
│   └── workflow.md
├── go.mod
└── Makefile
```

## Build & Test

```bash
# Build
make build

# Test
make test

# Lint
go vet ./...
```

## Coding Standards

- Keep functions short, single responsibility
- Errors must be handled, no `_ = err`
- Public APIs must have comments
- Do not introduce unused dependencies
- Variable names should be clear, no abbreviations

## Backlog Protocol

This project uses `backlog.json` (index) + `backlog/STORY-N.json` (details) for requirements management, replacing the traditional `.context/` file approach.

### File Structure

```
backlog.json              <- Lightweight index: id/title/status/branch/merge_commit only
backlog/
├── STORY-1.json          <- Full story details, agent only needs to read this one file
├── STORY-2.json
└── ...
```

### Rules

1. **Lightweight index**: `backlog.json` only stores summary info (id/title/status/branch/merge_commit) for quick browsing
2. **Independent details**: Each story's full data lives in `backlog/STORY-N.json`, agent only needs to read one file
3. **Dual-write sync**: When modifying story status, update both the index and the detail file
4. **Inline tasks**: Task breakdowns are stored in the `tasks` array within the story file, no separate files
5. **Reason required**: `reason` field is mandatory for file changes and design deviations
6. **Append-only audit_log**: Fully traceable, no deletion or modification of existing log entries
7. **Independent branches**: Each story uses its own feature branch, merged directly after review

### Status Flow

```
backlog → ready → designing → implementing → reviewing/testing → done
```

> Full JSON schemas and agent permissions: see `.claude/backlog-schema.md`
