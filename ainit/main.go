package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/*
var templateFS embed.FS

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot find home directory: %v\n", err)
		os.Exit(1)
	}

	claudeDir := filepath.Join(homeDir, ".claude")

	// 1. Install slash command
	cmdDir := filepath.Join(claudeDir, "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := installFile("templates/commands/ainit.md", filepath.Join(cmdDir, "ainit.md")); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ~/.claude/commands/ainit.md")

	// 2. Install template files
	templateDir := filepath.Join(claudeDir, "ainit-templates")
	if err := installDir("templates/agents", filepath.Join(templateDir, "agents")); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	for _, name := range []string{"workflow.md", "backlog-protocol.md"} {
		if err := installFile("templates/"+name, filepath.Join(templateDir, name)); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}

	if err := installFile("templates/backlog.mjs", filepath.Join(templateDir, "backlog.mjs")); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ~/.claude/ainit-templates/")

	fmt.Println()
	fmt.Println("Installed. Run /ainit in any project to set up multi-agent collaboration.")
}

func installFile(embedPath, destPath string) error {
	data, err := fs.ReadFile(templateFS, embedPath)
	if err != nil {
		return fmt.Errorf("read embedded %s: %w", embedPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(destPath, data, 0644)
}

func installDir(embedDir, destDir string) error {
	entries, err := fs.ReadDir(templateFS, embedDir)
	if err != nil {
		return fmt.Errorf("read embedded dir %s: %w", embedDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		src := embedDir + "/" + entry.Name()
		dst := filepath.Join(destDir, entry.Name())
		if err := installFile(src, dst); err != nil {
			return err
		}
	}
	return nil
}
