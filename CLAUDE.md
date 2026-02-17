# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

Monorepo containing two independent Go CLI tools for an AI agent platform:

- **agix/** — LLM reverse proxy that tracks tokens/cost, enforces per-agent budgets, and injects shared MCP tools transparently. Single binary, zero CGO.
- **ainit/** — Installs a `/ainit` slash command and agent templates into `~/.claude/` to enable multi-agent collaboration (team-lead, architect, coder, tester, reviewer, docs-sync) in any project.

Each tool is self-contained with its own `go.mod`, `Makefile`, and `CLAUDE.md` with detailed architecture docs. Work from within the tool's directory.

## Build & Test

Both tools use identical Makefile targets:

```bash
# From within agix/ or ainit/
make build          # Build binary
make install        # Build + install to /usr/local/bin
make test           # go test ./...
make vet            # go vet ./...
make clean          # Remove binary

# Run a single test
go test -v -run TestName ./internal/store/
```

## Code Conventions

- Go standard layout: `cmd/` for CLI commands (cobra), `internal/` for private packages
- Error wrapping: `fmt.Errorf("context: %w", err)`
- No globals: config and dependencies passed via struct fields
- Table-driven tests: `[]struct{ name string; ... }` pattern
- No CGO: pure Go only, for cross-compilation


## Development Workflow (MANDATORY)

When receiving a new feature request or change request, **DO NOT modify code directly**. You MUST follow this workflow:

1. Read `backlog.json` to get the current `last_story_id`, increment it for the new story
2. Create a story entry in `backlog.json` index and a `backlog/STORY-N.json` detail file, status set to `ready`
3. Create a feature branch: `git checkout -b feat/STORY-N-slug`
4. Progress the story through: `ready → designing → implementing → reviewing/testing → done`
5. Each status transition must be recorded in `audit_log` and synced to both `backlog.json` and `backlog/STORY-N.json`

**Code changes without a registered backlog story are NOT allowed.**

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


<!-- ainit:backlog-protocol -->
## Development Workflow (MANDATORY)

When receiving a new feature request or change request, **DO NOT modify code directly**. You MUST follow this workflow:

1. Read `backlog.json` to get the current `last_story_id`, increment it for the new story
2. Create a story entry in `backlog.json` index and a `backlog/STORY-N.json` detail file, status set to `ready`
3. Create a feature branch: `git checkout -b feat/STORY-N-slug`
4. Progress the story through: `ready → designing → implementing → reviewing/testing → done`
5. Each status transition must be recorded in `audit_log` and synced to both `backlog.json` and `backlog/STORY-N.json`

**Code changes without a registered backlog story are NOT allowed.**

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
