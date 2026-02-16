Initialize this project for multi-agent collaboration. Follow these steps exactly:

## Step 1: Run the setup script

Run `bash ~/.claude/ainit-templates/ainit-setup.sh` in the project root. Supports `--dry-run` (preview without changes). Templates are always refreshed; user data (backlog.json, CLAUDE.md) is protected. This script automatically:
- Copies agent templates to `.claude/agents/`
- Installs `.claude/backlog.mjs` (zero-dependency backlog CLI)
- Creates `backlog.json` and `backlog/` directory
- Copies `.claude/workflow.md`
- Appends backlog protocol to `CLAUDE.md` (if it exists)

If the script reports that `CLAUDE.md` was not found, run `/init` first to generate it, then re-run the script.

## Step 2: Customize templates for this project

Analyze the project and edit the installed agent templates to match the actual tech stack. Follow these sub-steps:

### 2a. Detect project context

Read these files to understand the project:
- `CLAUDE.md` — tech stack, build commands, coding standards
- `go.mod`, `package.json`, `pyproject.toml`, `Cargo.toml`, `pom.xml`, `build.gradle` — language & dependencies
- Existing test files (use Glob: `**/*_test.go`, `**/*.test.ts`, `**/test_*.py`, etc.) — test framework & patterns
- `Makefile`, `justfile`, `package.json scripts` — build/test/lint commands

### 2b. Edit agent templates

Using the Edit tool, customize each agent file in `.claude/agents/`:

**tester.md**:
- Replace the generic "Language-Specific Test Commands" table with the **single concrete command** for this project (e.g., `make test` or `go test ./...` or `npm test`)
- Add the actual test file naming convention found in the project (e.g., `*_test.go`, `*.spec.ts`)
- If a specific test framework is detected (jest, vitest, pytest, testing package, etc.), mention it by name

**coder.md**:
- Replace the generic "Build Verification" table with the **single concrete command** for this project
- Add any project-specific lint/format commands if found in CLAUDE.md or Makefile

**build-resolver.md**:
- Replace the generic "Build Commands by Language" table with the actual build & dependency commands for this project

**reviewer.md** and **security-reviewer.md**:
- Keep only the language-specific checks section relevant to this project's language, remove the others
- If the project has specific security requirements or patterns mentioned in CLAUDE.md, add them

**architect.md**:
- No changes needed (already language-agnostic by design)

**team-lead.md**:
- No changes needed (orchestration logic is language-agnostic)

### 2c. Edit workflow.md

Edit `.claude/workflow.md`:
- Update the "Model Tier Strategy" table if the project has specific preferences
- Add any project-specific conventions under a new "## Project-Specific Notes" section at the end

### Important rules for customization

- **Only remove generic content, never remove core workflow logic** (status flow, CLI commands, field formats, principles)
- **Be concrete**: replace "detect the build system" with the actual command
- **Be conservative**: if unsure about a convention, keep the generic version
- **Do not invent**: only add information you found in the project files

## Step 3: Confirm

Print a summary of:
1. Files installed by the setup script
2. Customizations made (which agents were edited, what was changed)
3. The project is now ready for multi-agent collaboration. Users can start by saying: "Use the team-lead agent to implement XXX feature".
