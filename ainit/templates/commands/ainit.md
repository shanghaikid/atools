Initialize this project for multi-agent collaboration. Follow these steps exactly:

## Step 1: Run the setup script

Run `bash ~/.claude/ainit-templates/ainit-setup.sh` in the project root. This script automatically:
- Copies agent templates to `.claude/agents/`
- Installs `backlog.mjs` (zero-dependency backlog CLI)
- Creates `backlog.json` and `backlog/` directory
- Copies `workflow.md`
- Appends backlog protocol to `CLAUDE.md` (if it exists)

If the script reports that `CLAUDE.md` was not found, run `/init` first to generate it, then re-run the script.

## Step 2: Confirm

Print a summary of the setup. The project is now ready for multi-agent collaboration. Users can start by saying: "Use the team-lead agent to implement XXX feature".
