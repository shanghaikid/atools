# Agile Multi-Agent Collaboration Workflow

## Overview

This template enables multi-agent collaboration through `backlog.json` (lightweight index) + `backlog/STORY-N.json` (independent details). Each story is self-contained with full lifecycle information and task breakdowns, and each feature uses an independent git branch + PR workflow.

## File Structure

```
backlog.json              <- Lightweight index: id/title/status/branch/pr_url
backlog/
├── STORY-1.json          <- Full details: requirements, tasks, design, implementation, review, testing, audit_log
├── STORY-2.json
└── ...
```

**Design principle**: An agent only needs to read one story file to get all context, no need to read the entire backlog.

## Architecture

```
User requirement
  │
  ▼
team-lead (opus) ──── orchestration, fully autonomous
  │
  ├─ Create story → backlog.json + backlog/STORY-N.json
  ├─ Create feature branch
  │
  ├─▶ architect (sonnet) ──▶ story.design + story.tasks
  │
  ├─▶ coder (sonnet) ──▶ story.implementation + task status + git commit/push
  │
  ├─ Create PR (gh pr create)
  │
  ├─▶ tester (sonnet) ──▶ story.testing     ┐
  ├─▶ reviewer (haiku) ──▶ story.review      ├ parallel
  │                                           ┘
  ├─ Decision: merge or rework
  │
  ├─▶ docs-sync (haiku) ──▶ update existing docs
  │
  └─ gh pr merge → story.status = done
```

## Usage

### 1. Configure the Project

Edit `CLAUDE.md` and fill in the `{{}}` placeholders.

Initialize `backlog.json`:
```json
{"project": "your-project-name", "current_sprint": 1, "last_story_id": 0, "stories": []}
```

### 2. Start a Task

```
Use the team-lead agent to implement XXX feature
```

team-lead will automatically:
1. Create story (index + detail file)
2. Create feature branch
3. Launch architect → write design + break down tasks
4. Launch coder → implement tasks → commit/push
5. Create PR
6. Launch tester + reviewer in parallel
7. Decide to merge or rework based on verdict
8. Merge PR, story status → done
9. Launch docs-sync
10. Summary report

### 3. Check Progress

```bash
# View all story statuses
cat backlog.json | jq '.stories[] | {id, title, status}'

# View a specific story's details
cat backlog/STORY-1.json | jq '{status, tasks: [.tasks[] | {id, title, status}]}'
```

## Story & Task Hierarchy

```
Story (user requirement)
├── Task 1 (implementation step)  ← architect breaks down
├── Task 2 (implementation step)  ← architect breaks down
├── Task 3 (testing task)         ← architect breaks down
└── ...
```

- **Story** = user-facing requirement, has its own branch + PR
- **Task** = implementation-side breakdown, inline in the story file, has assignee and status
- architect creates tasks, coder/tester update task status

## Story Lifecycle

### Status Flow

```
backlog → ready → designing → implementing → reviewing/testing → done
                                    ↑                │
                                    └── rework ──────┘
```

### Phase Descriptions

| Status | Meaning | Operator |
|--------|---------|----------|
| backlog | Waiting in the requirement pool | team-lead creates |
| ready | Broken down, awaiting design | team-lead confirms |
| designing | Architect designing | architect |
| implementing | Coding in progress | coder |
| reviewing/testing | Review + testing in progress | reviewer + tester |
| done | Completed and merged | team-lead merges PR |

## Branch & PR Workflow

1. **Create branch**: `git checkout -b feat/STORY-{id}-{slug}`
2. **Implement & commit**: coder commits + pushes on the feature branch
3. **Create PR**: `gh pr create --title "STORY-{id}: {title}"`
4. **Review & test**: reviewer uses `gh pr diff` to review, tester runs tests on the branch
5. **Merge**: `gh pr merge --squash`

## Model Tier Strategy

| Agent | Model | Reason |
|-------|-------|--------|
| team-lead | opus | Complex orchestration decisions require strongest reasoning |
| architect | sonnet | Design requires strong analytical ability |
| coder | sonnet | Coding requires accuracy |
| tester | sonnet | Test writing requires code understanding |
| reviewer | haiku | Checklist-style review, haiku is sufficient |
| docs-sync | haiku | Simple text updates, haiku is sufficient |

## Token Conservation Strategy

1. **File splitting**: Agent only reads one story file (tens to hundreds of lines), not the entire backlog
2. **Lightweight index**: backlog.json only stores summaries, team-lead browses quickly without burning tokens
3. **Read-only restrictions**: architect/reviewer cannot edit code files
4. **Avoid broadcast**: team-lead only sends point-to-point messages
5. **Model tiers**: Low-complexity tasks use haiku

## Rework Mechanism

- **Test failure** → story status back to `implementing`, append audit_log → relaunch coder
- **Critical review issue** → same as above
- **Only Warning/Suggestion** → record but do not rework
- **Maximum 2 rework rounds**, escalate to user if exceeded

## Copying to Another Project

1. Copy the following files to the target project root:
   - `CLAUDE.md`
   - `.claude/agents/`
   - `backlog.json`
   - `backlog/` (empty directory)
   - `workflow.md`
2. Edit `CLAUDE.md` to fill in project information
3. Edit `backlog.json` to fill in the project name
4. Start using
