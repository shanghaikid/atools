#!/usr/bin/env node
// Backlog CLI — zero-dependency Node.js script for managing backlog.json + backlog/STORY-N.json
// Usage: node backlog.mjs <command> [args]

import { readFileSync, writeFileSync, mkdirSync, existsSync, rmdirSync, statSync } from 'fs';
import { join, dirname } from 'path';

// ── Helpers ──

function lock(root) {
  const lockDir = join(root, '.backlog.lock');
  const maxRetries = 50;
  const retryDelay = 100; // ms
  for (let i = 0; i < maxRetries; i++) {
    try {
      mkdirSync(lockDir); // atomic — fails if already exists
      return;
    } catch (e) {
      if (e.code !== 'EEXIST') throw e;
      // Stale lock detection: if lock is older than 30s, remove it
      try {
        const stat = statSync(lockDir);
        if (Date.now() - stat.mtimeMs > 30000) {
          try { rmdirSync(lockDir); } catch (_) {}
          continue;
        }
      } catch (_) {}
      const sleepUntil = Date.now() + retryDelay;
      while (Date.now() < sleepUntil) {} // busy-wait (no async in this script)
    }
  }
  fatal('could not acquire lock after 5s — another backlog.mjs process may be stuck. Remove .backlog.lock manually if needed.');
}

function unlock(root) {
  try { rmdirSync(join(root, '.backlog.lock')); } catch (_) {}
}

function deepMerge(target, source) {
  if (typeof target !== 'object' || target === null || typeof source !== 'object' || source === null) {
    return source;
  }
  if (Array.isArray(target) && Array.isArray(source)) {
    return target.concat(source);
  }
  const result = { ...target };
  for (const key of Object.keys(source)) {
    if (key in result && typeof result[key] === 'object' && result[key] !== null && typeof source[key] === 'object' && source[key] !== null) {
      result[key] = deepMerge(result[key], source[key]);
    } else {
      result[key] = source[key];
    }
  }
  return result;
}

function findRoot() {
  let dir = process.cwd();
  while (dir !== dirname(dir)) {
    if (existsSync(join(dir, 'backlog.json'))) return dir;
    dir = dirname(dir);
  }
  fatal('backlog.json not found (searched upward from cwd)');
}

function fatal(msg) {
  console.error(JSON.stringify({ error: msg }));
  process.exit(1);
}

function readJSON(path) {
  try {
    return JSON.parse(readFileSync(path, 'utf8'));
  } catch (e) {
    fatal(`cannot read ${path}: ${e.message}`);
  }
}

function writeJSON(path, data) {
  writeFileSync(path, JSON.stringify(data, null, 2) + '\n');
}

function storyPath(root, id) {
  return join(root, 'backlog', `${id}.json`);
}

function loadIndex(root) {
  return readJSON(join(root, 'backlog.json'));
}

function saveIndex(root, index) {
  writeJSON(join(root, 'backlog.json'), index);
}

function loadStory(root, id) {
  return readJSON(storyPath(root, id));
}

function saveStory(root, id, story) {
  writeJSON(storyPath(root, id), story);
}

function normalizeId(raw) {
  return raw.toUpperCase().startsWith('STORY-') ? raw.toUpperCase() : `STORY-${raw}`;
}

function parseFlags(args) {
  const flags = {};
  const positional = [];
  let i = 0;
  while (i < args.length) {
    if (args[i].startsWith('--')) {
      const key = args[i].slice(2);
      // Collect all values until the next flag
      const values = [];
      i++;
      while (i < args.length && !args[i].startsWith('--')) {
        values.push(args[i]);
        i++;
      }
      flags[key] = values.length === 1 ? values[0] : values.length === 0 ? true : values;
    } else {
      positional.push(args[i]);
      i++;
    }
  }
  return { flags, positional };
}

function now() {
  return new Date().toISOString();
}

// ── Commands ──

function cmdList(root) {
  const index = loadIndex(root);
  console.log(JSON.stringify(index.stories));
}

function cmdShow(root, args) {
  if (!args[0]) fatal('usage: backlog show <id>');
  const id = normalizeId(args[0]);
  const story = loadStory(root, id);
  console.log(JSON.stringify(story));
}

function cmdCreate(root, args) {
  const { flags } = parseFlags(args);
  if (!flags.title) fatal('--title is required');
  if (!flags.desc) fatal('--desc is required');

  lock(root);
  try {
    const index = loadIndex(root);
    const newId = index.last_story_id + 1;
    const id = `STORY-${newId}`;
    // Slug uses STORY-N prefix in branch name, guaranteeing uniqueness even if titles collide
    const slug = String(flags.title).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '').slice(0, 40);
    const branch = `feat/${id}-${slug}`;
    const priority = flags.priority || 'medium';
    const criteria = Array.isArray(flags.criteria) ? flags.criteria : flags.criteria ? [flags.criteria] : [];

    const story = {
      id,
      title: flags.title,
      description: flags.desc,
      priority,
      sprint: index.current_sprint,
      status: 'ready',
      branch,
      merge_commit: null,
      acceptance_criteria: criteria,
      tasks: [],
      design: null,
      implementation: null,
      review: null,
      security_review: null,
      testing: null,
      audit_log: [
        { timestamp: now(), agent: 'team-lead', action: 'story_created', detail: flags.title }
      ]
    };

    mkdirSync(join(root, 'backlog'), { recursive: true });
    saveStory(root, id, story);

    index.last_story_id = newId;
    index.stories.push({ id, title: flags.title, status: 'ready', branch, merge_commit: null });
    saveIndex(root, index);

    console.log(JSON.stringify({ id, branch }));
  } finally {
    unlock(root);
  }
}

function cmdStatus(root, args) {
  if (args.length < 2) fatal('usage: backlog status <id> <new-status>');
  const id = normalizeId(args[0]);
  const newStatus = args[1];
  const valid = ['backlog', 'ready', 'designing', 'implementing', 'reviewing', 'testing', 'done'];
  if (!valid.includes(newStatus)) fatal(`invalid status: ${newStatus}. Valid: ${valid.join(', ')}`);

  lock(root);
  try {
    const story = loadStory(root, id);
    story.status = newStatus;
    saveStory(root, id, story);

    const index = loadIndex(root);
    const entry = index.stories.find(s => s.id === id);
    if (entry) entry.status = newStatus;
    saveIndex(root, index);

    console.log(JSON.stringify({ id, status: newStatus }));
  } finally {
    unlock(root);
  }
}

function cmdMergeCommit(root, args) {
  if (args.length < 2) fatal('usage: backlog merge-commit <id> <hash>');
  const id = normalizeId(args[0]);
  const hash = args[1];

  lock(root);
  try {
    const story = loadStory(root, id);
    story.merge_commit = hash;
    saveStory(root, id, story);

    const index = loadIndex(root);
    const entry = index.stories.find(s => s.id === id);
    if (entry) entry.merge_commit = hash;
    saveIndex(root, index);

    console.log(JSON.stringify({ id, merge_commit: hash }));
  } finally {
    unlock(root);
  }
}

function cmdSet(root, args) {
  // Parse --merge flag from anywhere in args
  const filteredArgs = [];
  let mergeMode = false;
  for (const a of args) {
    if (a === '--merge') { mergeMode = true; } else { filteredArgs.push(a); }
  }
  if (filteredArgs.length < 3) fatal('usage: backlog set <id> <field> \'<json>\' [--merge]');
  const id = normalizeId(filteredArgs[0]);
  const field = filteredArgs[1];
  const allowed = ['design', 'implementation', 'review', 'security_review', 'testing'];
  if (!allowed.includes(field)) fatal(`field must be one of: ${allowed.join(', ')}`);

  let value;
  try {
    value = JSON.parse(filteredArgs[2]);
  } catch (e) {
    fatal(`invalid JSON for ${field}: ${e.message}`);
  }

  // Default to merge mode for review and security_review to avoid overwrites
  const shouldMerge = mergeMode || field === 'review' || field === 'security_review';

  lock(root);
  try {
    const story = loadStory(root, id);
    if (shouldMerge && story[field] != null && typeof story[field] === 'object') {
      story[field] = deepMerge(story[field], value);
    } else {
      story[field] = value;
    }
    saveStory(root, id, story);

    console.log(JSON.stringify({ id, field, merged: shouldMerge, ok: true }));
  } finally {
    unlock(root);
  }
}

function cmdAddTask(root, args) {
  const idRaw = args[0];
  if (!idRaw) fatal('usage: backlog add-task <id> --title "..." --assignee coder --desc "..."');
  const id = normalizeId(idRaw);
  const { flags } = parseFlags(args.slice(1));
  if (!flags.title) fatal('--title is required');

  lock(root);
  try {
    const story = loadStory(root, id);
    const taskNum = story.tasks.length + 1;
    const task = {
      id: `TASK-${taskNum}`,
      title: flags.title,
      status: 'pending',
      assignee: flags.assignee || 'coder',
      description: flags.desc || ''
    };
    story.tasks.push(task);
    saveStory(root, id, story);

    console.log(JSON.stringify({ id, task_id: task.id }));
  } finally {
    unlock(root);
  }
}

function cmdTaskStatus(root, args) {
  if (args.length < 3) fatal('usage: backlog task-status <id> <task-id> <new-status>');
  const id = normalizeId(args[0]);
  const taskId = args[1].toUpperCase();
  const newStatus = args[2];
  const valid = ['pending', 'in_progress', 'done'];
  if (!valid.includes(newStatus)) fatal(`invalid task status: ${newStatus}. Valid: ${valid.join(', ')}`);

  lock(root);
  try {
    const story = loadStory(root, id);
    const task = story.tasks.find(t => t.id === taskId);
    if (!task) fatal(`task ${taskId} not found in ${id}`);
    task.status = newStatus;
    saveStory(root, id, story);

    console.log(JSON.stringify({ id, task_id: taskId, status: newStatus }));
  } finally {
    unlock(root);
  }
}

function cmdLog(root, args) {
  const idRaw = args[0];
  if (!idRaw) fatal('usage: backlog log <id> --agent team-lead --action "..." --detail "..."');
  const id = normalizeId(idRaw);
  const { flags } = parseFlags(args.slice(1));
  if (!flags.agent) fatal('--agent is required');
  if (!flags.action) fatal('--action is required');

  lock(root);
  try {
    const story = loadStory(root, id);
    if (!story.audit_log) story.audit_log = [];
    story.audit_log.push({
      timestamp: now(),
      agent: flags.agent,
      action: flags.action,
      detail: flags.detail || ''
    });
    saveStory(root, id, story);

    console.log(JSON.stringify({ id, logged: true }));
  } finally {
    unlock(root);
  }
}

// ── Main ──

const args = process.argv.slice(2);
const cmd = args[0];
const rest = args.slice(1);

if (!cmd) {
  console.log(`Usage: backlog <command> [args]

Commands:
  list                                         List all stories
  show <id>                                    Show story details
  create --title "..." --desc "..." [--priority high] [--criteria "c1" "c2"]
  status <id> <new-status>                     Update story status (dual-write)
  merge-commit <id> <hash>                     Set merge commit (dual-write)
  set <id> <field> '<json>' [--merge]            Set design/implementation/review/security_review/testing
  add-task <id> --title "..." [--assignee coder] [--desc "..."]
  task-status <id> <task-id> <new-status>      Update task status
  log <id> --agent <name> --action "..." [--detail "..."]`);
  process.exit(0);
}

const root = findRoot();

const commands = {
  list: () => cmdList(root),
  show: () => cmdShow(root, rest),
  create: () => cmdCreate(root, rest),
  status: () => cmdStatus(root, rest),
  'merge-commit': () => cmdMergeCommit(root, rest),
  set: () => cmdSet(root, rest),
  'add-task': () => cmdAddTask(root, rest),
  'task-status': () => cmdTaskStatus(root, rest),
  log: () => cmdLog(root, rest),
};

if (!commands[cmd]) fatal(`unknown command: ${cmd}`);
commands[cmd]();
