package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ProjectInfo holds metadata detected from an existing project directory.
type ProjectInfo struct {
	Name         string
	Description  string
	LangKey      string
	BuildCmd     string
	TestCmd      string
	LintCmd      string
	DirStructure string
}

// detectProject scans the target directory for existing project files
// and returns pre-filled metadata.
func detectProject(targetDir string) ProjectInfo {
	info := ProjectInfo{}

	// Detect language and extract metadata from marker files
	if detectGoProject(targetDir, &info) {
		info.LangKey = "go"
	} else if detectNodeProject(targetDir, &info) {
		info.LangKey = "ts"
	} else if detectPythonProject(targetDir, &info) {
		info.LangKey = "python"
	}

	// Detect directory structure
	info.DirStructure = scanDirStructure(targetDir)

	return info
}

// detectGoProject checks for go.mod and extracts module name.
func detectGoProject(targetDir string, info *ProjectInfo) bool {
	goModPath := filepath.Join(targetDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return false
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			modulePath := strings.TrimPrefix(line, "module ")
			// Use the last segment of the module path as project name
			parts := strings.Split(modulePath, "/")
			info.Name = parts[len(parts)-1]
			break
		}
	}

	return true
}

// detectNodeProject checks for package.json and extracts name, description, scripts.
func detectNodeProject(targetDir string, info *ProjectInfo) bool {
	pkgPath := filepath.Join(targetDir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return false
	}

	var pkg struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Scripts     map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return true // file exists but can't parse — still a node project
	}

	if pkg.Name != "" {
		info.Name = pkg.Name
	}
	if pkg.Description != "" {
		info.Description = pkg.Description
	}

	if scripts := pkg.Scripts; scripts != nil {
		if cmd, ok := scripts["build"]; ok {
			info.BuildCmd = "npm run build" // show npm command, not raw script
			_ = cmd
		}
		if _, ok := scripts["test"]; ok {
			info.TestCmd = "npm test"
		}
		if _, ok := scripts["lint"]; ok {
			info.LintCmd = "npm run lint"
		}
	}

	return true
}

// detectPythonProject checks for pyproject.toml, setup.py, or requirements.txt.
func detectPythonProject(targetDir string, info *ProjectInfo) bool {
	// Check pyproject.toml first (most modern)
	pyprojectPath := filepath.Join(targetDir, "pyproject.toml")
	if data, err := os.ReadFile(pyprojectPath); err == nil {
		parsePyproject(string(data), info)
		return true
	}

	// Check setup.py
	if _, err := os.Stat(filepath.Join(targetDir, "setup.py")); err == nil {
		return true
	}

	// Check requirements.txt
	if _, err := os.Stat(filepath.Join(targetDir, "requirements.txt")); err == nil {
		return true
	}

	return false
}

// parsePyproject extracts name and description from pyproject.toml.
// Simple line-based parsing to avoid adding a TOML dependency.
func parsePyproject(content string, info *ProjectInfo) {
	inProject := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[project]" || trimmed == "[tool.poetry]" {
			inProject = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			inProject = false
			continue
		}
		if !inProject {
			continue
		}
		if strings.HasPrefix(trimmed, "name") {
			info.Name = extractTomlString(trimmed)
		}
		if strings.HasPrefix(trimmed, "description") {
			info.Description = extractTomlString(trimmed)
		}
	}
}

// extractTomlString extracts a quoted string value from a TOML key = "value" line.
func extractTomlString(line string) string {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) < 2 {
		return ""
	}
	val := strings.TrimSpace(parts[1])
	val = strings.Trim(val, "\"'")
	return val
}

// scanDirStructure scans top-level subdirectories and generates a tree string.
func scanDirStructure(targetDir string) string {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return ""
	}

	// Collect visible directories (skip hidden dirs and common non-code dirs)
	skipDirs := map[string]bool{
		"node_modules": true,
		".git":         true,
		".claude":      true,
		"__pycache__":  true,
		".venv":        true,
		"venv":         true,
		"dist":         true,
		"build":        true,
		".next":        true,
		"backlog":      true,
	}

	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if skipDirs[name] {
			continue
		}
		if strings.HasPrefix(name, ".") {
			continue
		}
		dirs = append(dirs, name)
	}

	if len(dirs) == 0 {
		return ""
	}

	sort.Strings(dirs)

	var lines []string
	for i, d := range dirs {
		prefix := "├── "
		if i == len(dirs)-1 {
			prefix = "└── "
		}
		lines = append(lines, prefix+d+"/")
	}

	return strings.Join(lines, "\n")
}

// confirmOverwrites checks for existing files and asks the user whether to overwrite.
// Returns a map of filenames to skip (true = skip).
func confirmOverwrites(scanner *bufio.Scanner, targetDir string) map[string]bool {
	skip := make(map[string]bool)

	filesToCheck := []struct {
		name    string
		warning string
	}{
		{"CLAUDE.md", ""},
		{"workflow.md", ""},
		{"backlog.json", " (WARNING: may contain existing story data)"},
	}

	for _, f := range filesToCheck {
		path := filepath.Join(targetDir, f.name)
		if _, err := os.Stat(path); err == nil {
			fmt.Println()
			label := fmt.Sprintf("%s already exists%s. Overwrite? [y/N]", f.name, f.warning)
			confirm := prompt(scanner, label, "N")
			if strings.ToLower(confirm) != "y" {
				skip[f.name] = true
				fmt.Printf("  Skipping %s\n", f.name)
			}
		}
	}

	return skip
}
