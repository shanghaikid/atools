# Backlog Schema Reference

> Detailed JSON schemas and agent permissions for the backlog protocol.
> For workflow rules and status flow, see the backlog protocol section in CLAUDE.md.

## Index Schema — `backlog.json`

```json
{
  "project": "project name",
  "current_sprint": 1,
  "last_story_id": 0,
  "last_epic_id": 0,
  "epics": [
    {
      "id": "EPIC-N",
      "title": "short title",
      "phase": 1,
      "status": "planned|active|done"
    }
  ],
  "stories": [
    {
      "id": "STORY-N",
      "title": "short title",
      "status": "backlog|ready|designing|implementing|reviewing|testing|done",
      "branch": "feat/STORY-N-slug",
      "merge_commit": null,
      "epic": "EPIC-N",
      "phase": 1
    }
  ]
}
```

> `epic` and `phase` are optional on stories. They are omitted for stories created before Epic support was added.

## Epic Detail Schema — `backlog/EPIC-N.json`

```json
{
  "id": "EPIC-N",
  "title": "short title",
  "description": "what this epic delivers and why",
  "phase": 1,
  "status": "planned|active|done",
  "stories": ["STORY-1", "STORY-2"],
  "audit_log": [{ "timestamp", "agent", "action", "detail" }]
}
```

### Epic Status Flow

```
planned → active → done
```

| Status | Meaning |
|--------|---------|
| planned | Epic defined, no Stories started yet |
| active | At least one Story is in progress |
| done | All Stories merged, Epic goal delivered |

## Story Detail Schema — `backlog/STORY-N.json`

```json
{
  "id": "STORY-N",
  "title": "short title",
  "description": "requirement description",
  "priority": "high|medium|low",
  "sprint": 1,
  "epic": "EPIC-N",
  "phase": 1,
  "status": "backlog|ready|designing|implementing|reviewing|testing|done",
  "branch": "feat/STORY-N-slug",
  "merge_commit": null,
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
  "security_review": { "findings", "verdict" },
  "testing": { "tests_added", "tests_passed", "tests_failed", "failures", "verdict" },
  "audit_log": [{ "timestamp", "agent", "action", "detail" }]
}
```

## Agent Read/Write Permissions

| Agent | Read | Write |
|-------|------|-------|
| team-lead | backlog.json + any story/epic file | backlog.json index + story top-level fields, tasks, audit_log |
| product-manager | vision.md + backlog.json + any epic/story file | vision.md, epic files, creates stories via backlog.mjs |
| architect | assigned story file | story.design, story.tasks |
| coder | assigned story file | story.implementation, story.tasks status |
| tester | assigned story file | story.testing |
| reviewer | assigned story file + branch diff | story.review |
| security-reviewer | assigned story file + branch diff | story.security_review |
| docs-sync | story files with status=done | does not write backlog, only updates docs |
