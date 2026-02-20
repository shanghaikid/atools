package bundle

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agent-platform/agix/internal/config"
)

//go:embed builtins/*.json
var builtinFS embed.FS

// Bundle represents a named set of MCP servers and agent defaults.
type Bundle struct {
	Name          string                        `json:"name"`
	Description   string                        `json:"description"`
	Servers       map[string]config.MCPServer   `json:"servers"`
	AgentDefaults map[string]config.AgentTools  `json:"agent_defaults"`
}

// BundleInfo holds display information about a bundle.
type BundleInfo struct {
	Name        string
	Description string
	Builtin     bool
	Installed   bool
}

// LoadBuiltins returns all built-in bundles embedded in the binary.
func LoadBuiltins() (map[string]*Bundle, error) {
	entries, err := builtinFS.ReadDir("builtins")
	if err != nil {
		return nil, fmt.Errorf("read builtin bundles: %w", err)
	}

	bundles := make(map[string]*Bundle)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := builtinFS.ReadFile("builtins/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read builtin %s: %w", entry.Name(), err)
		}

		var b Bundle
		if err := json.Unmarshal(data, &b); err != nil {
			return nil, fmt.Errorf("parse builtin %s: %w", entry.Name(), err)
		}

		bundles[b.Name] = &b
	}

	return bundles, nil
}

// BundlesDir returns the path to the user bundles directory (~/.agix/bundles/).
func BundlesDir() (string, error) {
	dir, err := config.DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bundles"), nil
}

// LoadUser returns all user-defined bundles from ~/.agix/bundles/.
func LoadUser() (map[string]*Bundle, error) {
	dir, err := BundlesDir()
	if err != nil {
		return nil, err
	}

	bundles := make(map[string]*Bundle)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return bundles, nil
		}
		return nil, fmt.Errorf("read bundles directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read bundle %s: %w", entry.Name(), err)
		}

		var b Bundle
		if err := json.Unmarshal(data, &b); err != nil {
			return nil, fmt.Errorf("parse bundle %s: %w", entry.Name(), err)
		}

		bundles[b.Name] = &b
	}

	return bundles, nil
}

// LoadAll returns all available bundles (built-in + user), with user bundles
// taking precedence when names collide.
func LoadAll() (map[string]*Bundle, error) {
	builtins, err := LoadBuiltins()
	if err != nil {
		return nil, err
	}

	user, err := LoadUser()
	if err != nil {
		return nil, err
	}

	// User bundles override built-ins
	for name, b := range user {
		builtins[name] = b
	}

	return builtins, nil
}

// Get loads a single bundle by name, checking user bundles first, then built-ins.
func Get(name string) (*Bundle, error) {
	all, err := LoadAll()
	if err != nil {
		return nil, err
	}

	b, ok := all[name]
	if !ok {
		return nil, fmt.Errorf("bundle %q not found", name)
	}

	return b, nil
}

// List returns information about all available bundles sorted by name.
func List(installedBundles []string) ([]BundleInfo, error) {
	builtins, err := LoadBuiltins()
	if err != nil {
		return nil, err
	}

	user, err := LoadUser()
	if err != nil {
		return nil, err
	}

	installed := make(map[string]bool)
	for _, name := range installedBundles {
		installed[name] = true
	}

	seen := make(map[string]bool)
	var infos []BundleInfo

	// Add built-ins
	for name, b := range builtins {
		seen[name] = true
		infos = append(infos, BundleInfo{
			Name:        name,
			Description: b.Description,
			Builtin:     true,
			Installed:   installed[name],
		})
	}

	// Add user bundles (may override built-in info)
	for name, b := range user {
		if seen[name] {
			// Update existing entry
			for i := range infos {
				if infos[i].Name == name {
					infos[i].Description = b.Description
					infos[i].Builtin = false // user override
					break
				}
			}
		} else {
			infos = append(infos, BundleInfo{
				Name:        name,
				Description: b.Description,
				Builtin:     false,
				Installed:   installed[name],
			})
		}
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	return infos, nil
}

// Install merges a bundle's servers and agent defaults into the config.
// Returns an error if the bundle is already installed.
func Install(cfg *config.Config, b *Bundle) error {
	// Check if already installed
	for _, name := range cfg.Bundles {
		if name == b.Name {
			return fmt.Errorf("bundle %q is already installed", b.Name)
		}
	}

	// Initialize maps if nil
	if cfg.Tools.Servers == nil {
		cfg.Tools.Servers = make(map[string]config.MCPServer)
	}
	if cfg.Tools.Agents == nil {
		cfg.Tools.Agents = make(map[string]config.AgentTools)
	}

	// Merge servers
	for name, server := range b.Servers {
		cfg.Tools.Servers[name] = server
	}

	// Merge agent defaults
	for name, tools := range b.AgentDefaults {
		cfg.Tools.Agents[name] = tools
	}

	// Track installed bundle
	cfg.Bundles = append(cfg.Bundles, b.Name)

	return nil
}

// Remove removes a bundle's servers and agent defaults from the config.
// Returns an error if the bundle is not installed.
func Remove(cfg *config.Config, b *Bundle) error {
	found := false
	var remaining []string
	for _, name := range cfg.Bundles {
		if name == b.Name {
			found = true
		} else {
			remaining = append(remaining, name)
		}
	}

	if !found {
		return fmt.Errorf("bundle %q is not installed", b.Name)
	}

	cfg.Bundles = remaining

	// Remove servers that came from this bundle
	for name := range b.Servers {
		delete(cfg.Tools.Servers, name)
	}

	// Remove agent defaults that came from this bundle
	for name := range b.AgentDefaults {
		delete(cfg.Tools.Agents, name)
	}

	return nil
}
