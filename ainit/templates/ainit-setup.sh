#!/usr/bin/env bash
set -euo pipefail

# ainit-setup.sh — Initialize a project for multi-agent collaboration.
# Copies agent templates, backlog CLI, workflow docs, and sets up backlog infrastructure.
# Usage: bash ~/.claude/ainit-templates/ainit-setup.sh [--force] [--dry-run] [project-root]

TEMPLATE_DIR="$HOME/.claude/ainit-templates"
FORCE=false
DRY_RUN=false

# --- Parse flags ---
POSITIONAL=()
for arg in "$@"; do
  case "$arg" in
    --force)  FORCE=true ;;
    --dry-run) DRY_RUN=true ;;
    *)        POSITIONAL+=("$arg") ;;
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

# --- safe_copy: skip existing files unless --force ---
safe_copy() {
  local src="$1"
  local dst="$2"
  if $DRY_RUN; then
    if [ -f "$dst" ] && ! $FORCE; then
      echo "  [dry-run] SKIP $dst (already exists)"
    elif [ -f "$dst" ] && $FORCE; then
      echo "  [dry-run] OVERWRITE $dst (backup → ${dst}.bak.*)"
    else
      echo "  [dry-run] COPY → $dst"
    fi
    return
  fi
  if [ -f "$dst" ]; then
    if $FORCE; then
      cp "$dst" "${dst}.bak.$(date +%s)"
      cp "$src" "$dst"
      echo "  $dst (backed up & overwritten)"
    else
      echo "  $dst (already exists, skipped)"
    fi
  else
    cp "$src" "$dst"
    echo "  $dst"
  fi
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

# --- Step 1: Copy agent files ---
mkdir -p .claude/agents
for f in "$TEMPLATE_DIR"/agents/*.md; do
  safe_copy "$f" ".claude/agents/$(basename "$f")"
done
if ! $DRY_RUN; then
  echo "  .claude/agents/ ($(ls .claude/agents/*.md 2>/dev/null | wc -l | tr -d ' ') agents)"
fi

# --- Step 2: Copy skills (reference docs) ---
if [ -d "$TEMPLATE_DIR/skills" ]; then
  mkdir -p .claude/skills
  for f in "$TEMPLATE_DIR"/skills/*.md; do
    [ -f "$f" ] && safe_copy "$f" ".claude/skills/$(basename "$f")"
  done
  if ! $DRY_RUN; then
    echo "  .claude/skills/ ($(ls .claude/skills/*.md 2>/dev/null | wc -l | tr -d ' ') skills)"
  fi
fi

# --- Step 3: Install backlog CLI (into .claude/) ---
safe_copy "$TEMPLATE_DIR/backlog.mjs" .claude/backlog.mjs

# --- Step 4: Create backlog infrastructure ---
if $DRY_RUN; then
  if [ ! -f backlog.json ]; then
    echo "  [dry-run] CREATE backlog.json"
  else
    echo "  [dry-run] SKIP backlog.json (already exists)"
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

# --- Step 5: Copy workflow.md (into .claude/) ---
safe_copy "$TEMPLATE_DIR/workflow.md" .claude/workflow.md

# --- Step 6: Update CLAUDE.md ---
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
  echo "Done! Project '$PROJECT_NAME' is ready for multi-agent collaboration."
  echo "Start by saying: \"Use the team-lead agent to implement XXX feature\""
fi
