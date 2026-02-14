package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed templates/*
var templateFS embed.FS

// Backlog is the backlog.json structure.
type Backlog struct {
	Project       string        `json:"project"`
	CurrentSprint int           `json:"current_sprint"`
	LastStoryID   int           `json:"last_story_id"`
	Stories       []interface{} `json:"stories"`
}

func main() {
	targetDir := "."
	if len(os.Args) > 1 {
		targetDir = os.Args[1]
	}

	// Resolve to absolute path
	absDir, err := filepath.Abs(targetDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	targetDir = absDir

	// Create target directory if it doesn't exist
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot create directory %s: %v\n", targetDir, err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("ainit — Multi-Agent Collaboration Project Initializer")
	fmt.Println()

	// Project name (needed for backlog.json)
	defaultName := filepath.Base(targetDir)
	detected := detectProject(targetDir)
	if detected.Name != "" {
		defaultName = detected.Name
	}
	projectName := prompt(scanner, "Project name", defaultName)

	// Check for existing files and confirm overwrite
	skipFiles := confirmOverwrites(scanner, targetDir)

	fmt.Println()

	// 1. Generate CLAUDE.md via claude CLI + append backlog protocol
	if !skipFiles["CLAUDE.md"] {
		if err := generateClaudeMD(targetDir); err != nil {
			fatal("failed to generate CLAUDE.md", err)
		}
	}

	// 2. Copy workflow.md
	if !skipFiles["workflow.md"] {
		if err := copyEmbedded(targetDir, "workflow.md", "templates/workflow.md"); err != nil {
			fatal("failed to generate workflow.md", err)
		}
		fmt.Println("  workflow.md")
	}

	// 3. Generate backlog.json
	if !skipFiles["backlog.json"] {
		if err := generateBacklog(targetDir, projectName); err != nil {
			fatal("failed to generate backlog.json", err)
		}
		fmt.Println("  backlog.json")
	}

	// 4. Create backlog/ directory
	backlogDir := filepath.Join(targetDir, "backlog")
	if err := os.MkdirAll(backlogDir, 0755); err != nil {
		fatal("failed to create backlog/", err)
	}
	fmt.Println("  backlog/")

	// 5. Copy agent files
	agents := []string{"team-lead", "architect", "coder", "tester", "reviewer", "docs-sync"}
	agentDir := filepath.Join(targetDir, ".claude", "agents")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		fatal("failed to create .claude/agents/", err)
	}
	for _, agent := range agents {
		src := "templates/agents/" + agent + ".md"
		dst := filepath.Join(".claude", "agents", agent+".md")
		if err := copyEmbedded(targetDir, dst, src); err != nil {
			fatal("failed to generate "+dst, err)
		}
		fmt.Println("  .claude/agents/" + agent + ".md")
	}

	fmt.Println()
	fmt.Println("Initialization complete!")
}

// generateClaudeMD uses the claude CLI to analyze the codebase and generate CLAUDE.md,
// then appends the backlog protocol section.
func generateClaudeMD(targetDir string) error {
	claudeMDPath := filepath.Join(targetDir, "CLAUDE.md")

	// Check if claude CLI is available
	if _, err := exec.LookPath("claude"); err != nil {
		fmt.Println("  claude CLI not found, skipping AI analysis.")
		fmt.Println("  Install from https://docs.anthropic.com/en/docs/claude-code")
		fmt.Println("  Generating CLAUDE.md with backlog protocol only...")
		// Write just the backlog protocol section
		protocol, err := fs.ReadFile(templateFS, "templates/backlog-protocol.md")
		if err != nil {
			return err
		}
		if err := os.WriteFile(claudeMDPath, protocol, 0644); err != nil {
			return err
		}
		fmt.Println("  CLAUDE.md")
		return nil
	}

	fmt.Print("  Generating CLAUDE.md with claude CLI...")

	promptText := `Analyze the codebase in the current directory and generate a CLAUDE.md file. Include:
- Project name and one-line description
- Tech stack (language, framework, database, other tools)
- Directory structure overview
- Build, test, and lint commands
- Coding standards and conventions found in the codebase
- Any important architectural patterns or project-specific notes

Output the complete CLAUDE.md content in markdown format. Do not wrap it in code fences.`

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "-p",
		"--output-format", "text",
		"--no-session-persistence",
		promptText,
	)
	cmd.Dir = targetDir

	output, err := cmd.Output()
	if err != nil {
		fmt.Println(" failed.")
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Println("  Timed out. Generating CLAUDE.md with backlog protocol only...")
		} else {
			fmt.Printf("  claude CLI error: %v\n", err)
			fmt.Println("  Generating CLAUDE.md with backlog protocol only...")
		}
		// Fallback: write just the backlog protocol
		protocol, readErr := fs.ReadFile(templateFS, "templates/backlog-protocol.md")
		if readErr != nil {
			return readErr
		}
		if writeErr := os.WriteFile(claudeMDPath, protocol, 0644); writeErr != nil {
			return writeErr
		}
		fmt.Println("  CLAUDE.md")
		return nil
	}

	fmt.Println(" done.")

	// Append backlog protocol to claude's output
	protocol, err := fs.ReadFile(templateFS, "templates/backlog-protocol.md")
	if err != nil {
		return err
	}

	content := strings.TrimSpace(string(output)) + "\n\n" + string(protocol)
	if err := os.WriteFile(claudeMDPath, []byte(content), 0644); err != nil {
		return err
	}

	fmt.Println("  CLAUDE.md")
	return nil
}

func prompt(scanner *bufio.Scanner, label, defaultVal string) string {
	fmt.Printf("%s [%s]: ", label, defaultVal)
	if scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input != "" {
			return input
		}
	}
	return defaultVal
}

func generateBacklog(targetDir, projectName string) error {
	backlog := Backlog{
		Project:       projectName,
		CurrentSprint: 1,
		LastStoryID:   0,
		Stories:       []interface{}{},
	}

	data, err := json.MarshalIndent(backlog, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(targetDir, "backlog.json"), data, 0644)
}

func copyEmbedded(targetDir, relPath, embedPath string) error {
	data, err := fs.ReadFile(templateFS, embedPath)
	if err != nil {
		return err
	}

	outPath := filepath.Join(targetDir, relPath)
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(outPath, data, 0644)
}

func fatal(msg string, err error) {
	fmt.Fprintf(os.Stderr, "error: %s: %v\n", msg, err)
	os.Exit(1)
}
