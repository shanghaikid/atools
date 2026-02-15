#!/usr/bin/env bash
set -euo pipefail

# ainit-setup.sh — Initialize a project for multi-agent collaboration.
# Copies agent templates, backlog CLI, workflow docs, and sets up backlog infrastructure.
# Usage: bash ~/.claude/ainit-templates/ainit-setup.sh [project-root]

TEMPLATE_DIR="$HOME/.claude/ainit-templates"
PROJECT_DIR="${1:-.}"
cd "$PROJECT_DIR"

# --- Detect project name ---
detect_project_name() {
  if [ -f package.json ]; then
    name=$(grep -o '"name"[[:space:]]*:[[:space:]]*"[^"]*"' package.json | head -1 | sed 's/.*"name"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')
    [ -n "$name" ] && echo "$name" && return
  fi
  if [ -f go.mod ]; then
    name=$(head -1 go.mod | awk '{print $2}' | awk -F/ '{print $NF}')
    [ -n "$name" ] && echo "$name" && return
  fi
  if [ -f pyproject.toml ]; then
    name=$(grep -o '^name[[:space:]]*=[[:space:]]*"[^"]*"' pyproject.toml | head -1 | sed 's/.*"\([^"]*\)"/\1/')
    [ -n "$name" ] && echo "$name" && return
  fi
  if [ -f Cargo.toml ]; then
    name=$(grep -o '^name[[:space:]]*=[[:space:]]*"[^"]*"' Cargo.toml | head -1 | sed 's/.*"\([^"]*\)"/\1/')
    [ -n "$name" ] && echo "$name" && return
  fi
  basename "$(pwd)"
}

PROJECT_NAME=$(detect_project_name)

# --- Step 1: Copy agent files ---
mkdir -p .claude/agents
for f in "$TEMPLATE_DIR"/agents/*.md; do
  cp "$f" .claude/agents/
done
echo "  .claude/agents/ ($(ls .claude/agents/*.md 2>/dev/null | wc -l | tr -d ' ') agents)"

# --- Step 2: Copy skills (reference docs) ---
if [ -d "$TEMPLATE_DIR/skills" ]; then
  mkdir -p .claude/skills
  for f in "$TEMPLATE_DIR"/skills/*.md; do
    [ -f "$f" ] && cp "$f" .claude/skills/
  done
  echo "  .claude/skills/ ($(ls .claude/skills/*.md 2>/dev/null | wc -l | tr -d ' ') skills)"
fi

# --- Step 3: Install backlog CLI ---
cp "$TEMPLATE_DIR/backlog.mjs" backlog.mjs
echo "  backlog.mjs"

# --- Step 4: Create backlog infrastructure ---
if [ ! -f backlog.json ]; then
  printf '{"project": "%s", "current_sprint": 1, "last_story_id": 0, "stories": []}\n' "$PROJECT_NAME" > backlog.json
  echo "  backlog.json (created)"
else
  echo "  backlog.json (already exists, skipped)"
fi
mkdir -p backlog
echo "  backlog/"

# --- Step 5: Copy workflow.md ---
cp "$TEMPLATE_DIR/workflow.md" workflow.md
echo "  workflow.md"

# --- Step 6: Update CLAUDE.md ---
if [ -f CLAUDE.md ]; then
  if grep -q "## Development Workflow (MANDATORY)" CLAUDE.md 2>/dev/null; then
    echo "  CLAUDE.md (backlog protocol already present, skipped)"
  else
    printf '\n\n' >> CLAUDE.md
    cat "$TEMPLATE_DIR/backlog-protocol.md" >> CLAUDE.md
    echo "  CLAUDE.md (appended backlog protocol)"
  fi
else
  echo "  CLAUDE.md not found — run /init first, then re-run this script to append backlog protocol"
fi

echo ""
echo "Done! Project '$PROJECT_NAME' is ready for multi-agent collaboration."
echo "Start by saying: \"Use the team-lead agent to implement XXX feature\""
