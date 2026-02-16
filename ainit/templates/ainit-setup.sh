#!/usr/bin/env bash
set -euo pipefail

# ainit-setup.sh — Initialize a project for multi-agent collaboration.
# Copies agent templates, backlog CLI, workflow docs, and sets up backlog infrastructure.
# Templates are always overwritten (Step 2 of /ainit customizes them per-project).
# User data (backlog.json, CLAUDE.md) is never overwritten.
# Usage: bash ~/.claude/ainit-templates/ainit-setup.sh [--dry-run] [project-root]

TEMPLATE_DIR="$HOME/.claude/ainit-templates"
DRY_RUN=false

# --- Parse flags ---
POSITIONAL=()
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
    *)         POSITIONAL+=("$arg") ;;
  esac
done
PROJECT_DIR="${POSITIONAL[0]:-.}"
cd "$PROJECT_DIR"

# --- Node.js check ---
if ! command -v node &>/dev/null; then
  echo "⚠  Warning: Node.js not found. backlog.mjs requires Node.js to run."
  echo "   Install Node.js from https://nodejs.org/ or via your package manager."
  echo ""
fi

# --- copy_template: always overwrite (templates are re-customized by Step 2) ---
copy_template() {
  local src="$1"
  local dst="$2"
  if $DRY_RUN; then
    if [ -f "$dst" ]; then
      echo "  [dry-run] UPDATE $dst"
    else
      echo "  [dry-run] COPY → $dst"
    fi
    return
  fi
  cp "$src" "$dst"
}

# --- Detect project name ---
detect_project_name() {
  if [ -f package.json ]; then
    if command -v node &>/dev/null; then
      name=$(node -e "try{console.log(JSON.parse(require('fs').readFileSync('package.json','utf8')).name||'')}catch(e){}" 2>/dev/null)
    else
      name=$(grep -o '"name"[[:space:]]*:[[:space:]]*"[^"]*"' package.json | head -1 | sed 's/.*"name"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')
    fi
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

# --- Step 1: Copy agent templates (always overwrite) ---
mkdir -p .claude/agents
for f in "$TEMPLATE_DIR"/agents/*.md; do
  copy_template "$f" ".claude/agents/$(basename "$f")"
done
if ! $DRY_RUN; then
  echo "  .claude/agents/ ($(ls .claude/agents/*.md 2>/dev/null | wc -l | tr -d ' ') agents)"
fi

# --- Step 2: Copy skills (always overwrite) ---
if [ -d "$TEMPLATE_DIR/skills" ]; then
  mkdir -p .claude/skills
  for f in "$TEMPLATE_DIR"/skills/*.md; do
    [ -f "$f" ] && copy_template "$f" ".claude/skills/$(basename "$f")"
  done
  if ! $DRY_RUN; then
    echo "  .claude/skills/ ($(ls .claude/skills/*.md 2>/dev/null | wc -l | tr -d ' ') skills)"
  fi
fi

# --- Step 3: Install backlog CLI (always overwrite) ---
copy_template "$TEMPLATE_DIR/backlog.mjs" .claude/backlog.mjs
if ! $DRY_RUN; then
  echo "  .claude/backlog.mjs"
fi

# --- Step 4: Create backlog infrastructure (protect user data) ---
if $DRY_RUN; then
  if [ ! -f backlog.json ]; then
    echo "  [dry-run] CREATE backlog.json"
  else
    echo "  [dry-run] SKIP backlog.json (user data, already exists)"
  fi
  echo "  [dry-run] MKDIR backlog/"
else
  if [ ! -f backlog.json ]; then
    printf '{"project": "%s", "current_sprint": 1, "last_story_id": 0, "stories": []}\n' "$PROJECT_NAME" > backlog.json
    echo "  backlog.json (created)"
  else
    echo "  backlog.json (already exists, skipped)"
  fi
  mkdir -p backlog
  echo "  backlog/"
fi

# --- Step 5: Copy workflow.md (always overwrite) ---
copy_template "$TEMPLATE_DIR/workflow.md" .claude/workflow.md
if ! $DRY_RUN; then
  echo "  .claude/workflow.md"
fi

# --- Step 6: Update CLAUDE.md (idempotent append, protect user content) ---
if $DRY_RUN; then
  if [ -f CLAUDE.md ]; then
    if grep -q '<!-- ainit:backlog-protocol -->' CLAUDE.md 2>/dev/null; then
      echo "  [dry-run] SKIP CLAUDE.md (backlog protocol already present)"
    else
      echo "  [dry-run] APPEND backlog protocol to CLAUDE.md"
    fi
  else
    echo "  [dry-run] CLAUDE.md not found"
  fi
else
  if [ -f CLAUDE.md ]; then
    if grep -q '<!-- ainit:backlog-protocol -->' CLAUDE.md 2>/dev/null; then
      echo "  CLAUDE.md (backlog protocol already present, skipped)"
    else
      printf '\n\n' >> CLAUDE.md
      cat "$TEMPLATE_DIR/backlog-protocol.md" >> CLAUDE.md
      echo "  CLAUDE.md (appended backlog protocol)"
    fi
  else
    echo "  CLAUDE.md not found — run /init first, then re-run this script to append backlog protocol"
  fi
fi

echo ""
if $DRY_RUN; then
  echo "Dry run complete. No files were modified."
else
  echo "Done! Project '$PROJECT_NAME' templates installed."
  echo "Step 2 of /ainit will now customize them for this project's tech stack."
fi
