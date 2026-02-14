package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectGoProject(t *testing.T) {
	dir := t.TempDir()
	goMod := `module github.com/example/myapp

go 1.22
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644)

	info := detectProject(dir)

	if info.LangKey != "go" {
		t.Errorf("expected LangKey=go, got %q", info.LangKey)
	}
	if info.Name != "myapp" {
		t.Errorf("expected Name=myapp, got %q", info.Name)
	}
}

func TestDetectNodeProject(t *testing.T) {
	dir := t.TempDir()
	pkgJSON := `{
  "name": "my-web-app",
  "description": "A cool web app",
  "scripts": {
    "build": "tsc",
    "test": "jest",
    "lint": "eslint ."
  }
}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644)

	info := detectProject(dir)

	if info.LangKey != "ts" {
		t.Errorf("expected LangKey=ts, got %q", info.LangKey)
	}
	if info.Name != "my-web-app" {
		t.Errorf("expected Name=my-web-app, got %q", info.Name)
	}
	if info.Description != "A cool web app" {
		t.Errorf("expected Description='A cool web app', got %q", info.Description)
	}
	if info.BuildCmd != "npm run build" {
		t.Errorf("expected BuildCmd='npm run build', got %q", info.BuildCmd)
	}
	if info.TestCmd != "npm test" {
		t.Errorf("expected TestCmd='npm test', got %q", info.TestCmd)
	}
	if info.LintCmd != "npm run lint" {
		t.Errorf("expected LintCmd='npm run lint', got %q", info.LintCmd)
	}
}

func TestDetectPythonProject(t *testing.T) {
	dir := t.TempDir()
	pyproject := `[project]
name = "my-python-lib"
description = "A Python library"
version = "1.0.0"
`
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0644)

	info := detectProject(dir)

	if info.LangKey != "python" {
		t.Errorf("expected LangKey=python, got %q", info.LangKey)
	}
	if info.Name != "my-python-lib" {
		t.Errorf("expected Name=my-python-lib, got %q", info.Name)
	}
	if info.Description != "A Python library" {
		t.Errorf("expected Description='A Python library', got %q", info.Description)
	}
}

func TestDetectPythonRequirementsTxt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask\n"), 0644)

	info := detectProject(dir)

	if info.LangKey != "python" {
		t.Errorf("expected LangKey=python, got %q", info.LangKey)
	}
}

func TestDetectNoProject(t *testing.T) {
	dir := t.TempDir()

	info := detectProject(dir)

	if info.LangKey != "" {
		t.Errorf("expected empty LangKey, got %q", info.LangKey)
	}
	if info.Name != "" {
		t.Errorf("expected empty Name, got %q", info.Name)
	}
}

func TestScanDirStructure(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "cmd"), 0755)
	os.MkdirAll(filepath.Join(dir, "internal"), 0755)
	os.MkdirAll(filepath.Join(dir, "pkg"), 0755)
	// Hidden and skip dirs should be excluded
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	os.MkdirAll(filepath.Join(dir, "node_modules"), 0755)

	result := scanDirStructure(dir)

	if !strings.Contains(result, "cmd/") {
		t.Errorf("expected cmd/ in result, got %q", result)
	}
	if !strings.Contains(result, "internal/") {
		t.Errorf("expected internal/ in result, got %q", result)
	}
	if !strings.Contains(result, "pkg/") {
		t.Errorf("expected pkg/ in result, got %q", result)
	}
	if strings.Contains(result, ".git") {
		t.Errorf("should not contain .git, got %q", result)
	}
	if strings.Contains(result, "node_modules") {
		t.Errorf("should not contain node_modules, got %q", result)
	}

	// Check tree formatting
	lines := strings.Split(result, "\n")
	lastLine := lines[len(lines)-1]
	if !strings.HasPrefix(lastLine, "└── ") {
		t.Errorf("last line should use └── prefix, got %q", lastLine)
	}
}

func TestScanDirStructureEmpty(t *testing.T) {
	dir := t.TempDir()

	result := scanDirStructure(dir)

	if result != "" {
		t.Errorf("expected empty string for empty dir, got %q", result)
	}
}

func TestNewDirectoryCreation(t *testing.T) {
	parent := t.TempDir()
	newDir := filepath.Join(parent, "brand-new-project")

	// Directory should not exist
	if _, err := os.Stat(newDir); err == nil {
		t.Fatal("directory should not exist yet")
	}

	// Simulate what main() does
	if err := os.MkdirAll(newDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	stat, err := os.Stat(newDir)
	if err != nil {
		t.Fatalf("directory was not created: %v", err)
	}
	if !stat.IsDir() {
		t.Fatal("created path is not a directory")
	}
}

func TestExtractTomlString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`name = "foo"`, "foo"},
		{`name = 'bar'`, "bar"},
		{`description = "hello world"`, "hello world"},
		{`name`, ""},
	}

	for _, tt := range tests {
		result := extractTomlString(tt.input)
		if result != tt.expected {
			t.Errorf("extractTomlString(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
