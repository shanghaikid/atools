package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Port != 8080 {
		t.Errorf("DefaultConfig().Port = %d, want 8080", cfg.Port)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("DefaultConfig().LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.Keys == nil {
		t.Error("DefaultConfig().Keys is nil, want non-nil map")
	}
	if cfg.Budgets == nil {
		t.Error("DefaultConfig().Budgets is nil, want non-nil map")
	}
	if cfg.Tools.MaxIterations != 10 {
		t.Errorf("DefaultConfig().Tools.MaxIterations = %d, want 10", cfg.Tools.MaxIterations)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	original := &Config{
		Port:     9090,
		Database: "/tmp/test.db",
		LogLevel: "debug",
		Keys: map[string]string{
			"openai":    "sk-test-key",
			"anthropic": "sk-ant-test-key",
		},
		Budgets: map[string]Budget{
			"agent-1": {
				DailyLimitUSD:   10.00,
				MonthlyLimitUSD: 100.00,
				AlertAtPercent:  80.0,
			},
		},
	}

	// Save
	if err := Save(path, original); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("Save() did not create file")
	}

	// Load
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.Port != original.Port {
		t.Errorf("Load().Port = %d, want %d", loaded.Port, original.Port)
	}
	if loaded.Database != original.Database {
		t.Errorf("Load().Database = %q, want %q", loaded.Database, original.Database)
	}
	if loaded.LogLevel != original.LogLevel {
		t.Errorf("Load().LogLevel = %q, want %q", loaded.LogLevel, original.LogLevel)
	}
	if loaded.Keys["openai"] != original.Keys["openai"] {
		t.Errorf("Load().Keys[openai] = %q, want %q", loaded.Keys["openai"], original.Keys["openai"])
	}
	if loaded.Keys["anthropic"] != original.Keys["anthropic"] {
		t.Errorf("Load().Keys[anthropic] = %q, want %q", loaded.Keys["anthropic"], original.Keys["anthropic"])
	}

	budget, ok := loaded.Budgets["agent-1"]
	if !ok {
		t.Fatal("Load().Budgets missing agent-1")
	}
	if budget.DailyLimitUSD != 10.00 {
		t.Errorf("budget.DailyLimitUSD = %f, want 10.00", budget.DailyLimitUSD)
	}
	if budget.MonthlyLimitUSD != 100.00 {
		t.Errorf("budget.MonthlyLimitUSD = %f, want 100.00", budget.MonthlyLimitUSD)
	}
	if budget.AlertAtPercent != 80.0 {
		t.Errorf("budget.AlertAtPercent = %f, want 80.0", budget.AlertAtPercent)
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Load() with nonexistent file should return error")
	}
}

func TestLoadMalformedYAML(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.yaml")

	if err := os.WriteFile(path, []byte(":::invalid yaml:::[\n"), 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("Load() with malformed YAML should return error")
	}
}

func TestSaveCreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nested", "deep", "config.yaml")

	cfg := &Config{Port: 8080}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("Save() did not create nested directories and file")
	}
}

func TestSaveFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	cfg := &Config{Port: 8080}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want 600", perm)
	}
}

func TestLoadPartialConfig(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "partial.yaml")

	// Only set port, rest should come from defaults
	content := []byte("port: 3000\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Port != 3000 {
		t.Errorf("cfg.Port = %d, want 3000", cfg.Port)
	}
	// Default values should be preserved
	if cfg.LogLevel != "info" {
		t.Errorf("cfg.LogLevel = %q, want %q (default)", cfg.LogLevel, "info")
	}
}

func TestLoadEmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.yaml")

	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Should get defaults for empty config
	if cfg.Port != 8080 {
		t.Errorf("cfg.Port = %d, want 8080 (default)", cfg.Port)
	}
}

func TestToolsConfig(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	cfg := &Config{
		Port: 8080,
		Tools: ToolsConfig{
			MaxIterations: 5,
			Servers: map[string]MCPServer{
				"filesystem": {
					Command: "npx",
					Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
				},
				"github": {
					Command: "npx",
					Args:    []string{"-y", "@modelcontextprotocol/server-github"},
					Env:     []string{"GITHUB_TOKEN=ghp_xxx"},
				},
			},
			Agents: map[string]AgentTools{
				"code-reviewer": {Allow: []string{"read_file", "list_directory"}},
				"docs-writer":   {Deny: []string{"write_file", "delete_file"}},
			},
		},
	}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.Tools.MaxIterations != 5 {
		t.Errorf("MaxIterations = %d, want 5", loaded.Tools.MaxIterations)
	}

	if len(loaded.Tools.Servers) != 2 {
		t.Fatalf("Servers count = %d, want 2", len(loaded.Tools.Servers))
	}

	fs := loaded.Tools.Servers["filesystem"]
	if fs.Command != "npx" {
		t.Errorf("filesystem.Command = %q, want npx", fs.Command)
	}
	if len(fs.Args) != 3 {
		t.Errorf("filesystem.Args len = %d, want 3", len(fs.Args))
	}

	gh := loaded.Tools.Servers["github"]
	if len(gh.Env) != 1 || gh.Env[0] != "GITHUB_TOKEN=ghp_xxx" {
		t.Errorf("github.Env = %v, want [GITHUB_TOKEN=ghp_xxx]", gh.Env)
	}

	reviewer := loaded.Tools.Agents["code-reviewer"]
	if len(reviewer.Allow) != 2 {
		t.Errorf("code-reviewer.Allow len = %d, want 2", len(reviewer.Allow))
	}
	if reviewer.Allow[0] != "read_file" {
		t.Errorf("code-reviewer.Allow[0] = %q, want read_file", reviewer.Allow[0])
	}

	writer := loaded.Tools.Agents["docs-writer"]
	if len(writer.Deny) != 2 {
		t.Errorf("docs-writer.Deny len = %d, want 2", len(writer.Deny))
	}
}

func TestToolsConfigEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	content := []byte("port: 8080\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// MaxIterations should be default 10
	if cfg.Tools.MaxIterations != 10 {
		t.Errorf("Tools.MaxIterations = %d, want 10 (default)", cfg.Tools.MaxIterations)
	}
	if cfg.Tools.Servers != nil {
		t.Errorf("Tools.Servers = %v, want nil", cfg.Tools.Servers)
	}
	if cfg.Tools.Agents != nil {
		t.Errorf("Tools.Agents = %v, want nil", cfg.Tools.Agents)
	}
}

func TestBudgetStruct(t *testing.T) {
	tests := []struct {
		name   string
		budget Budget
	}{
		{
			name:   "zero budget",
			budget: Budget{},
		},
		{
			name: "daily only",
			budget: Budget{
				DailyLimitUSD: 5.00,
			},
		},
		{
			name: "monthly only",
			budget: Budget{
				MonthlyLimitUSD: 100.00,
			},
		},
		{
			name: "all fields",
			budget: Budget{
				DailyLimitUSD:   10.00,
				MonthlyLimitUSD: 200.00,
				AlertAtPercent:  90.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "config.yaml")

			cfg := &Config{
				Port: 8080,
				Budgets: map[string]Budget{
					"test-agent": tt.budget,
				},
			}

			if err := Save(path, cfg); err != nil {
				t.Fatalf("Save() error: %v", err)
			}

			loaded, err := Load(path)
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}

			got := loaded.Budgets["test-agent"]
			if got.DailyLimitUSD != tt.budget.DailyLimitUSD {
				t.Errorf("DailyLimitUSD = %f, want %f", got.DailyLimitUSD, tt.budget.DailyLimitUSD)
			}
			if got.MonthlyLimitUSD != tt.budget.MonthlyLimitUSD {
				t.Errorf("MonthlyLimitUSD = %f, want %f", got.MonthlyLimitUSD, tt.budget.MonthlyLimitUSD)
			}
			if got.AlertAtPercent != tt.budget.AlertAtPercent {
				t.Errorf("AlertAtPercent = %f, want %f", got.AlertAtPercent, tt.budget.AlertAtPercent)
			}
		})
	}
}
