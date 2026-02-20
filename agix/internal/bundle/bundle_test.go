package bundle

import (
	"testing"

	"github.com/agent-platform/agix/internal/config"
)

func TestLoadBuiltins(t *testing.T) {
	builtins, err := LoadBuiltins()
	if err != nil {
		t.Fatalf("LoadBuiltins() error: %v", err)
	}

	expected := []string{"code-review", "devops", "docs-writer"}
	for _, name := range expected {
		b, ok := builtins[name]
		if !ok {
			t.Errorf("missing built-in bundle %q", name)
			continue
		}
		if b.Name != name {
			t.Errorf("bundle name = %q, want %q", b.Name, name)
		}
		if b.Description == "" {
			t.Errorf("bundle %q has empty description", name)
		}
		if len(b.Servers) == 0 {
			t.Errorf("bundle %q has no servers", name)
		}
	}

	if len(builtins) != len(expected) {
		t.Errorf("LoadBuiltins() returned %d bundles, want %d", len(builtins), len(expected))
	}
}

func TestInstall(t *testing.T) {
	tests := []struct {
		name    string
		bundle  *Bundle
		cfg     *config.Config
		wantErr bool
	}{
		{
			name: "install into empty config",
			bundle: &Bundle{
				Name:        "test-bundle",
				Description: "Test bundle",
				Servers: map[string]config.MCPServer{
					"fs": {Command: "npx", Args: []string{"-y", "server-fs"}},
				},
				AgentDefaults: map[string]config.AgentTools{
					"agent1": {Allow: []string{"read_file"}},
				},
			},
			cfg:     &config.Config{},
			wantErr: false,
		},
		{
			name: "install duplicate fails",
			bundle: &Bundle{
				Name:    "dup",
				Servers: map[string]config.MCPServer{},
			},
			cfg: &config.Config{
				Bundles: []string{"dup"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Install(tt.cfg, tt.bundle)
			if (err != nil) != tt.wantErr {
				t.Errorf("Install() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Verify servers merged
			for name := range tt.bundle.Servers {
				if _, ok := tt.cfg.Tools.Servers[name]; !ok {
					t.Errorf("server %q not found in config after install", name)
				}
			}

			// Verify agent defaults merged
			for name := range tt.bundle.AgentDefaults {
				if _, ok := tt.cfg.Tools.Agents[name]; !ok {
					t.Errorf("agent %q not found in config after install", name)
				}
			}

			// Verify bundle tracked
			found := false
			for _, name := range tt.cfg.Bundles {
				if name == tt.bundle.Name {
					found = true
				}
			}
			if !found {
				t.Error("bundle name not tracked in config.Bundles")
			}
		})
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name    string
		bundle  *Bundle
		cfg     *config.Config
		wantErr bool
	}{
		{
			name: "remove installed bundle",
			bundle: &Bundle{
				Name: "test-bundle",
				Servers: map[string]config.MCPServer{
					"fs": {Command: "npx"},
				},
				AgentDefaults: map[string]config.AgentTools{
					"agent1": {Allow: []string{"read_file"}},
				},
			},
			cfg: &config.Config{
				Bundles: []string{"test-bundle"},
				Tools: config.ToolsConfig{
					Servers: map[string]config.MCPServer{
						"fs": {Command: "npx"},
					},
					Agents: map[string]config.AgentTools{
						"agent1": {Allow: []string{"read_file"}},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "remove not-installed fails",
			bundle: &Bundle{
				Name:    "missing",
				Servers: map[string]config.MCPServer{},
			},
			cfg:     &config.Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Remove(tt.cfg, tt.bundle)
			if (err != nil) != tt.wantErr {
				t.Errorf("Remove() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Verify servers removed
			for name := range tt.bundle.Servers {
				if _, ok := tt.cfg.Tools.Servers[name]; ok {
					t.Errorf("server %q still in config after remove", name)
				}
			}

			// Verify agent defaults removed
			for name := range tt.bundle.AgentDefaults {
				if _, ok := tt.cfg.Tools.Agents[name]; ok {
					t.Errorf("agent %q still in config after remove", name)
				}
			}

			// Verify bundle untracked
			for _, name := range tt.cfg.Bundles {
				if name == tt.bundle.Name {
					t.Error("bundle name still tracked in config.Bundles")
				}
			}
		})
	}
}

func TestList(t *testing.T) {
	infos, err := List([]string{"code-review"})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(infos) < 3 {
		t.Fatalf("List() returned %d bundles, want at least 3", len(infos))
	}

	// Should be sorted by name
	for i := 1; i < len(infos); i++ {
		if infos[i].Name < infos[i-1].Name {
			t.Errorf("bundles not sorted: %q after %q", infos[i].Name, infos[i-1].Name)
		}
	}

	// Check installed flag
	for _, info := range infos {
		if info.Name == "code-review" && !info.Installed {
			t.Error("code-review should be marked as installed")
		}
		if info.Name == "devops" && info.Installed {
			t.Error("devops should not be marked as installed")
		}
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		name    string
		bundle  string
		wantErr bool
	}{
		{"existing builtin", "code-review", false},
		{"missing bundle", "nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := Get(tt.bundle)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get(%q) error = %v, wantErr %v", tt.bundle, err, tt.wantErr)
				return
			}
			if !tt.wantErr && b.Name != tt.bundle {
				t.Errorf("Get(%q) returned name %q", tt.bundle, b.Name)
			}
		})
	}
}

func TestInstallThenRemove(t *testing.T) {
	b := &Bundle{
		Name: "roundtrip",
		Servers: map[string]config.MCPServer{
			"srv1": {Command: "cmd1"},
			"srv2": {Command: "cmd2"},
		},
		AgentDefaults: map[string]config.AgentTools{
			"a1": {Allow: []string{"tool1"}},
		},
	}

	cfg := &config.Config{}

	if err := Install(cfg, b); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	if len(cfg.Tools.Servers) != 2 {
		t.Errorf("after install: %d servers, want 2", len(cfg.Tools.Servers))
	}
	if len(cfg.Bundles) != 1 {
		t.Errorf("after install: %d tracked bundles, want 1", len(cfg.Bundles))
	}

	if err := Remove(cfg, b); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if len(cfg.Tools.Servers) != 0 {
		t.Errorf("after remove: %d servers, want 0", len(cfg.Tools.Servers))
	}
	if len(cfg.Bundles) != 0 {
		t.Errorf("after remove: %d tracked bundles, want 0", len(cfg.Bundles))
	}
}
