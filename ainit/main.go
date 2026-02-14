package main

import (
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/*
var templateFS embed.FS

// TemplateData holds all variables for CLAUDE.md template rendering.
type TemplateData struct {
	ProjectName  string
	Description  string
	Language     string
	Framework    string
	Database     string
	Other        string
	DirStructure string
	BuildCmd     string
	TestCmd      string
	LintCmd      string
}

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

	// Detect existing project metadata
	detected := detectProject(targetDir)

	// Project name
	dirName := filepath.Base(targetDir)
	defaultName := dirName
	if detected.Name != "" {
		defaultName = detected.Name
	}
	projectName := prompt(scanner, "Project name", defaultName)

	// Description
	defaultDesc := detected.Description
	var description string
	if defaultDesc != "" {
		description = prompt(scanner, "Short description", defaultDesc)
	} else {
		description = promptRequired(scanner, "Short description")
	}

	fmt.Println()
	fmt.Println("Tech stack:")

	// Language — detect defaults from existing project or fallback to go
	langKey := detected.LangKey
	if langKey == "" {
		langKey = "go"
	}
	defaults := getDefaults(langKey)
	language := prompt(scanner, "  Language", defaults.Language)

	// Re-detect defaults based on actual language input
	langKey = detectLang(language)
	defaults = getDefaults(langKey)

	framework := prompt(scanner, "  Framework", defaults.Framework)
	database := prompt(scanner, "  Database (enter None if N/A)", defaults.Database)
	other := prompt(scanner, "  Other tools", defaults.Other)

	fmt.Println()
	fmt.Println("Directory structure (one entry per line, blank line to finish, press Enter for default):")
	// Use detected directory structure if available, otherwise use language defaults
	defaultDirStructure := defaults.DirStructure
	if detected.DirStructure != "" {
		defaultDirStructure = detected.DirStructure
	}
	dirStructure := promptMultiline(scanner, defaultDirStructure)

	fmt.Println()
	fmt.Println("Build & test:")
	// Use detected commands if available, otherwise use language defaults
	defaultBuild := defaults.BuildCmd
	if detected.BuildCmd != "" {
		defaultBuild = detected.BuildCmd
	}
	defaultTest := defaults.TestCmd
	if detected.TestCmd != "" {
		defaultTest = detected.TestCmd
	}
	defaultLint := defaults.LintCmd
	if detected.LintCmd != "" {
		defaultLint = detected.LintCmd
	}
	buildCmd := prompt(scanner, "  Build command", defaultBuild)
	testCmd := prompt(scanner, "  Test command", defaultTest)
	lintCmd := prompt(scanner, "  Lint command", defaultLint)

	// Check for existing files and confirm overwrite
	skipFiles := confirmOverwrites(scanner, targetDir)

	data := TemplateData{
		ProjectName:  projectName,
		Description:  description,
		Language:     language,
		Framework:    framework,
		Database:     database,
		Other:        other,
		DirStructure: dirStructure,
		BuildCmd:     buildCmd,
		TestCmd:      testCmd,
		LintCmd:      lintCmd,
	}

	fmt.Println()
	fmt.Println("Generated files:")

	// 1. Generate CLAUDE.md from template
	if !skipFiles["CLAUDE.md"] {
		if err := generateClaudeMD(targetDir, data); err != nil {
			fatal("failed to generate CLAUDE.md", err)
		}
		fmt.Println("  CLAUDE.md")
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

func promptRequired(scanner *bufio.Scanner, label string) string {
	for {
		fmt.Printf("%s: ", label)
		if scanner.Scan() {
			input := strings.TrimSpace(scanner.Text())
			if input != "" {
				return input
			}
		}
		fmt.Println("  (required)")
	}
}

func promptMultiline(scanner *bufio.Scanner, defaultVal string) string {
	fmt.Print("> ")
	if !scanner.Scan() {
		return defaultVal
	}
	firstLine := strings.TrimSpace(scanner.Text())
	if firstLine == "" {
		return defaultVal
	}

	var lines []string
	lines = append(lines, firstLine)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			break
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func generateClaudeMD(targetDir string, data TemplateData) error {
	tmplBytes, err := templateFS.ReadFile("templates/CLAUDE.md.tmpl")
	if err != nil {
		return err
	}

	tmpl, err := template.New("CLAUDE.md").Delims("[[", "]]").Parse(string(tmplBytes))
	if err != nil {
		return err
	}

	outPath := filepath.Join(targetDir, "CLAUDE.md")
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
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
