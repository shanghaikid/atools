package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agent-platform/tools/ainit/internal/installer"
)

//go:embed templates/*
var templateFS embed.FS

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	dryRun := flag.Bool("dry-run", false, "show what would be installed without writing files")
	flag.Parse()

	if *showVersion {
		fmt.Println("ainit " + version)
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot find home directory: %v\n", err)
		os.Exit(1)
	}

	inst := &installer.Installer{FS: templateFS, DryRun: *dryRun}
	claudeDir := filepath.Join(homeDir, ".claude")

	// 1. Install slash command
	cmdDir := filepath.Join(claudeDir, "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := inst.InstallFile("templates/commands/ainit.md", filepath.Join(cmdDir, "ainit.md")); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ~/.claude/commands/ainit.md")

	// 2. Install template files
	templateDir := filepath.Join(claudeDir, "ainit-templates")
	if err := inst.InstallDir("templates/agents", filepath.Join(templateDir, "agents")); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := inst.InstallDir("templates/skills", filepath.Join(templateDir, "skills")); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	for _, name := range []string{"workflow.md", "backlog-protocol.md", "backlog-schema.md", "backlog.mjs", "ainit-setup.sh"} {
		if err := inst.InstallFile("templates/"+name, filepath.Join(templateDir, name)); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
	// Make setup script executable
	if !*dryRun {
		if err := os.Chmod(filepath.Join(templateDir, "ainit-setup.sh"), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Println("  ~/.claude/ainit-templates/")

	fmt.Println()
	if *dryRun {
		fmt.Println("Dry run complete. No files were written.")
	} else {
		fmt.Println("Installed. Run /ainit in any project to set up multi-agent collaboration.")
	}
}
