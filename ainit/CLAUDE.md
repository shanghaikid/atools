# ainit

> Multi-Agent Collaboration Project Initializer

## Tech Stack

- Language: Go 1.22
- Framework: Gin
- Database: None
- Other: No

## Directory Structure

```
├── cmd/            # entrypoint
├── internal/       # business logic
├── pkg/            # shared packages
├── api/            # API definitions
└── tests/          # tests
```

## Build & Test

```bash
# Build
go build -o app ./cmd/...

# Test
go test ./...

# Lint
golangci-lint run
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
backlog.json              <- Lightweight index: id/title/status/branch/pr_url only
backlog/
├── STORY-1.json          <- Full story details, agent only needs to read this one file
├── STORY-2.json
└── ...
```

### Index Schema — `backlog.json`

```json
{
  "project": "project name",
  "current_sprint": 1,
  "last_story_id": 0,
  "stories": [
    {
      "id": "STORY-N",
      "title": "short title",
      "status": "backlog|ready|designing|implementing|reviewing|testing|done",
      "branch": "feat/STORY-N-slug",
      "pr_url": null
    }
  ]
}
```

### Story Detail Schema — `backlog/STORY-N.json`

```json
{
  "id": "STORY-N",
  "title": "short title",
  "description": "requirement description",
  "priority": "high|medium|low",
  "sprint": 1,
  "status": "backlog|ready|designing|implementing|reviewing|testing|done",
  "branch": "feat/STORY-N-slug",
  "pr_url": null,
  "acceptance_criteria": ["criteria 1", "criteria 2"],

  "tasks": [
    {
      "id": "TASK-1",
      "title": "task title",
      "status": "pending|in_progress|done",
      "assignee": "architect|coder|tester|reviewer",
      "description": "what to do"
    }
  ],

  "design": { "summary", "files_involved", "decisions", "steps" },
  "implementation": { "changes", "build_status", "deviations" },
  "review": { "findings", "verdict" },
  "testing": { "tests_added", "tests_passed", "tests_failed", "failures", "verdict" },
  "audit_log": [{ "timestamp", "agent", "action", "detail" }]
}
```

### Rules

1. **Lightweight index**: `backlog.json` only stores summary info (id/title/status/branch/pr_url) for quick browsing
2. **Independent details**: Each story's full data lives in `backlog/STORY-N.json`, agent only needs to read one file
3. **Dual-write sync**: When modifying story status, update both the index and the detail file
4. **Inline tasks**: Task breakdowns are stored in the `tasks` array within the story file, no separate files
5. **Reason required**: `reason` field is mandatory for file changes and design deviations
6. **Append-only audit_log**: Fully traceable, no deletion or modification of existing log entries
7. **Independent branches**: Each story uses its own feature branch + PR workflow

### Agent Read/Write Permissions

| Agent | Read | Write |
|-------|------|-------|
| team-lead | backlog.json + any story file | backlog.json index + story top-level fields, tasks, audit_log |
| architect | assigned story file | story.design, story.tasks |
| coder | assigned story file | story.implementation, story.tasks status |
| tester | assigned story file | story.testing |
| reviewer | assigned story file + PR diff | story.review |
| docs-sync | story files with status=done | does not write backlog, only updates docs |

### Status Flow

```
backlog → ready → designing → implementing → reviewing/testing → done
```
